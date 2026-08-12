// Package config defines Kelyro's format-independent configuration schema,
// validation, precedence, and persistence contracts.
package config

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

const SchemaVersion = 1

const (
	KeyUIColor          = "ui.color"
	KeyEditorCommand    = "editor.command"
	KeyEditorPrompt     = "editor.prompt"
	KeyAllowNetwork     = "privacy.allow_network"
	KeyAllowAIContent   = "privacy.allow_ai_content"
	KeyAllowTelemetry   = "privacy.allow_usage_telemetry"
	KeyUpdateCheck      = "updates.check"
	KeyWorkspaceName    = "workspace.name"
	KeyMasteryThreshold = "learning.mastery_threshold"
	KeyBackupRetention  = "backup.retention"
)

// Scope identifies the file in which a configuration value is persisted.
type Scope string

const (
	ScopeGlobal  Scope = "global"
	ScopeProject Scope = "project"
)

// Kind identifies the scalar type accepted by a configuration key.
type Kind uint8

const (
	String Kind = iota
	Boolean
	Number
)

// Value is a validated scalar configuration value.
type Value struct {
	kind    Kind
	stringV string
	boolV   bool
	numberV float64
}

func StringValue(value string) Value  { return Value{kind: String, stringV: value} }
func BoolValue(value bool) Value      { return Value{kind: Boolean, boolV: value} }
func NumberValue(value float64) Value { return Value{kind: Number, numberV: value} }

func (value Value) Kind() Kind { return value.kind }
func (value Value) String() string {
	switch value.kind {
	case String:
		return value.stringV
	case Boolean:
		return strconv.FormatBool(value.boolV)
	case Number:
		return strconv.FormatFloat(value.numberV, 'f', -1, 64)
	default:
		return "<invalid>"
	}
}

func (value Value) StringField() (string, bool)  { return value.stringV, value.kind == String }
func (value Value) BoolField() (bool, bool)      { return value.boolV, value.kind == Boolean }
func (value Value) NumberField() (float64, bool) { return value.numberV, value.kind == Number }

// Settings holds an explicit layer or a fully resolved configuration.
type Settings map[string]Value

// Definition describes a known setting. Common settings are suitable for a
// future interactive settings wizard; every setting remains editable by file.
type Definition struct {
	Kind        Kind
	ProjectOnly bool
	Common      bool
}

var definitions = map[string]Definition{
	KeyUIColor:          {Kind: String, Common: true},
	KeyEditorCommand:    {Kind: String, Common: true},
	KeyEditorPrompt:     {Kind: Boolean, Common: true},
	KeyAllowNetwork:     {Kind: Boolean, Common: true},
	KeyAllowAIContent:   {Kind: Boolean, Common: true},
	KeyAllowTelemetry:   {Kind: Boolean, Common: true},
	KeyUpdateCheck:      {Kind: Boolean, Common: true},
	KeyWorkspaceName:    {Kind: String, ProjectOnly: true, Common: true},
	KeyMasteryThreshold: {Kind: Number, ProjectOnly: true, Common: true},
	KeyBackupRetention:  {Kind: Number},
}

// Defaults returns a fresh copy of Kelyro's safe default settings.
func Defaults() Settings {
	return Settings{
		KeyUIColor:          StringValue("auto"),
		KeyEditorCommand:    StringValue(""),
		KeyEditorPrompt:     BoolValue(true),
		KeyAllowNetwork:     BoolValue(false),
		KeyAllowAIContent:   BoolValue(false),
		KeyAllowTelemetry:   BoolValue(false),
		KeyUpdateCheck:      BoolValue(true),
		KeyWorkspaceName:    StringValue(""),
		KeyMasteryThreshold: NumberValue(0.85),
		KeyBackupRetention:  NumberValue(5),
	}
}

// Definitions returns a copy of the supported schema for presentation
// adapters, including a future settings wizard.
func Definitions() map[string]Definition {
	result := make(map[string]Definition, len(definitions))
	for key, definition := range definitions {
		result[key] = definition
	}
	return result
}

// Keys returns every supported dotted key in stable order.
func Keys() []string {
	keys := make([]string, 0, len(definitions))
	for key := range definitions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// CommonKeys returns settings suitable for the minimal interactive wizard.
func CommonKeys() []string {
	keys := make([]string, 0, len(definitions))
	for key, definition := range definitions {
		if definition.Common {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

// ParseValue converts CLI or file input into the type required by key.
func ParseValue(key, input string) (Value, error) {
	definition, ok := definitions[key]
	if !ok {
		return Value{}, fmt.Errorf("unknown configuration key %q", key)
	}

	var value Value
	switch definition.Kind {
	case String:
		value = StringValue(input)
	case Boolean:
		parsed, err := strconv.ParseBool(input)
		if err != nil || (input != "true" && input != "false") {
			return Value{}, fmt.Errorf("configuration key %q expects true or false", key)
		}
		value = BoolValue(parsed)
	case Number:
		parsed, err := strconv.ParseFloat(input, 64)
		if err != nil {
			return Value{}, fmt.Errorf("configuration key %q expects a number", key)
		}
		value = NumberValue(parsed)
	default:
		return Value{}, fmt.Errorf("configuration key %q has an invalid schema", key)
	}

	if err := validateValue(key, value); err != nil {
		return Value{}, err
	}
	return value, nil
}

// ValidateLayer validates known keys, scalar types, values, and scope.
func ValidateLayer(settings Settings, scope Scope) error {
	if scope != ScopeGlobal && scope != ScopeProject {
		return fmt.Errorf("invalid configuration scope %q", scope)
	}
	for key, value := range settings {
		definition, ok := definitions[key]
		if !ok {
			return fmt.Errorf("unknown configuration key %q", key)
		}
		if scope == ScopeGlobal && definition.ProjectOnly {
			return fmt.Errorf("configuration key %q is only valid in project configuration", key)
		}
		if value.kind != definition.Kind {
			return fmt.Errorf("configuration key %q has the wrong value type", key)
		}
		if err := validateValue(key, value); err != nil {
			return err
		}
	}
	return nil
}

func validateValue(key string, value Value) error {
	switch key {
	case KeyUIColor:
		if value.stringV != "auto" && value.stringV != "always" && value.stringV != "never" {
			return fmt.Errorf("configuration key %q expects auto, always, or never", key)
		}
	case KeyWorkspaceName:
		if strings.TrimSpace(value.stringV) == "" {
			return fmt.Errorf("configuration key %q must not be empty", key)
		}
	case KeyMasteryThreshold:
		if math.IsNaN(value.numberV) || math.IsInf(value.numberV, 0) || value.numberV <= 0 || value.numberV > 1 {
			return fmt.Errorf("configuration key %q must be greater than 0 and at most 1", key)
		}
	case KeyBackupRetention:
		if math.IsNaN(value.numberV) || math.IsInf(value.numberV, 0) || value.numberV < 1 || value.numberV > 100 || math.Trunc(value.numberV) != value.numberV {
			return fmt.Errorf("configuration key %q must be an integer from 1 to 100", key)
		}
	}
	return nil
}

// Resolve applies layers from least to most specific and validates the result.
func Resolve(layers ...Settings) (Settings, error) {
	resolved := Defaults()
	for _, layer := range layers {
		for key, value := range layer {
			definition, ok := definitions[key]
			if !ok {
				return nil, fmt.Errorf("unknown configuration key %q", key)
			}
			if value.kind != definition.Kind {
				return nil, fmt.Errorf("configuration key %q has the wrong value type", key)
			}
			if err := validateValue(key, value); err != nil {
				return nil, err
			}
			resolved[key] = value
		}
	}
	return resolved, nil
}

// Store persists validated settings at global and project scope. Set methods
// permit adapters to preserve comments while updating one key atomically.
type Store interface {
	GlobalPath() (string, error)
	ProjectPath(root string) (string, error)
	LoadGlobal() (Settings, error)
	LoadProject(root string) (Settings, error)
	SaveGlobal(settings Settings) error
	SaveProject(root string, settings Settings) error
	SetGlobal(key string, value Value) error
	SetProject(root, key string, value Value) error
}
