// Package configfs persists Kelyro configuration as TOML files on the local
// filesystem.
package configfs

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/platform"
)

const maxConfigSize = 1024 * 1024

// Store implements layered TOML persistence. The parser intentionally accepts
// only the scalar TOML forms used by Kelyro's strict schema.
type Store struct {
	globalPath  func() (string, error)
	projectPath func(string) (string, error)
	readFile    func(string) ([]byte, error)
	rename      func(string, string) error
}

// New creates a configuration store using Kelyro's native global and workspace
// paths.
func New() *Store {
	return &Store{
		globalPath:  platform.GlobalConfigPath,
		projectPath: platform.WorkspaceConfigPath,
		readFile:    os.ReadFile,
		rename:      os.Rename,
	}
}

func (store *Store) GlobalPath() (string, error) { return store.globalPath() }
func (store *Store) ProjectPath(root string) (string, error) {
	return store.projectPath(root)
}

func (store *Store) LoadGlobal() (config.Settings, error) {
	path, err := store.GlobalPath()
	if err != nil {
		return nil, err
	}
	return store.load(path, config.ScopeGlobal)
}

func (store *Store) LoadProject(root string) (config.Settings, error) {
	path, err := store.ProjectPath(root)
	if err != nil {
		return nil, err
	}
	return store.load(path, config.ScopeProject)
}

func (store *Store) SaveGlobal(settings config.Settings) error {
	path, err := store.GlobalPath()
	if err != nil {
		return err
	}
	return store.save(path, config.ScopeGlobal, settings)
}

func (store *Store) SaveProject(root string, settings config.Settings) error {
	path, err := store.ProjectPath(root)
	if err != nil {
		return err
	}
	return store.save(path, config.ScopeProject, settings)
}

func (store *Store) SetGlobal(key string, value config.Value) error {
	path, err := store.GlobalPath()
	if err != nil {
		return err
	}
	return store.set(path, config.ScopeGlobal, key, value)
}

func (store *Store) SetProject(root, key string, value config.Value) error {
	path, err := store.ProjectPath(root)
	if err != nil {
		return err
	}
	return store.set(path, config.ScopeProject, key, value)
}

func (store *Store) load(path string, scope config.Scope) (config.Settings, error) {
	encoded, err := store.readFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return config.Settings{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s configuration %s: %w", scope, path, err)
	}
	if len(encoded) > maxConfigSize {
		return nil, fmt.Errorf("read %s configuration %s: file exceeds %d bytes", scope, path, maxConfigSize)
	}

	settings, err := parse(encoded, scope)
	if err != nil {
		return nil, fmt.Errorf("invalid %s configuration %s: %w", scope, path, err)
	}
	return settings, nil
}

func (store *Store) save(path string, scope config.Scope, settings config.Settings) error {
	if err := config.ValidateLayer(settings, scope); err != nil {
		return err
	}
	return store.writeAtomic(path, encode(settings))
}

func (store *Store) set(path string, scope config.Scope, key string, value config.Value) error {
	settings, err := store.load(path, scope)
	if err != nil {
		return err
	}
	settings[key] = value
	if err := config.ValidateLayer(settings, scope); err != nil {
		return err
	}

	existing, err := store.readFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return store.writeAtomic(path, encode(settings))
	}
	if err != nil {
		return fmt.Errorf("read %s configuration %s for update: %w", scope, path, err)
	}
	return store.writeAtomic(path, updateDocument(existing, key, value))
}

func (store *Store) writeAtomic(path string, encoded []byte) (err error) {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create configuration directory %s: %w", directory, err)
	}

	temporary, err := os.CreateTemp(directory, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create configuration staging file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()

	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("secure configuration staging file: %w", err)
	}
	if _, err := temporary.Write(encoded); err != nil {
		return fmt.Errorf("write configuration staging file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync configuration staging file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close configuration staging file: %w", err)
	}
	if err := store.rename(temporaryPath, path); err != nil {
		return fmt.Errorf("commit configuration file %s: %w", path, err)
	}

	return nil
}

func parse(encoded []byte, scope config.Scope) (config.Settings, error) {
	settings := config.Settings{}
	section := ""
	sections := map[string]bool{}
	knownSections := map[string]bool{}
	for key := range config.Definitions() {
		name, _, _ := strings.Cut(key, ".")
		knownSections[name] = true
	}
	schemaVersion := config.SchemaVersion
	schemaSeen := false

	scanner := bufio.NewScanner(strings.NewReader(string(encoded)))
	scanner.Buffer(make([]byte, 4096), maxConfigSize)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(withoutComment(scanner.Text()))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") || strings.HasPrefix(line, "[[") {
				return nil, fmt.Errorf("line %d: invalid table header", lineNumber)
			}
			section = strings.TrimSpace(line[1 : len(line)-1])
			if section == "" || strings.ContainsAny(section, "[] .\t") {
				return nil, fmt.Errorf("line %d: invalid table name %q", lineNumber, section)
			}
			if !knownSections[section] {
				return nil, fmt.Errorf("line %d: unknown configuration table %q", lineNumber, section)
			}
			if sections[section] {
				return nil, fmt.Errorf("line %d: duplicate table %q", lineNumber, section)
			}
			sections[section] = true
			continue
		}

		equals := strings.IndexByte(line, '=')
		if equals < 1 {
			return nil, fmt.Errorf("line %d: expected key = value", lineNumber)
		}
		name := strings.TrimSpace(line[:equals])
		raw := strings.TrimSpace(line[equals+1:])
		if name == "" || raw == "" || strings.ContainsAny(name, " .\t") {
			return nil, fmt.Errorf("line %d: invalid assignment", lineNumber)
		}

		if section == "" {
			if name != "schema_version" {
				return nil, fmt.Errorf("line %d: unknown top-level key %q", lineNumber, name)
			}
			if schemaSeen {
				return nil, fmt.Errorf("line %d: duplicate key %q", lineNumber, name)
			}
			parsed, err := strconv.Atoi(raw)
			if err != nil {
				return nil, fmt.Errorf("line %d: schema_version expects an integer", lineNumber)
			}
			schemaVersion, schemaSeen = parsed, true
			continue
		}

		key := section + "." + name
		if _, exists := settings[key]; exists {
			return nil, fmt.Errorf("line %d: duplicate key %q", lineNumber, key)
		}
		definition, exists := config.Definitions()[key]
		if !exists {
			return nil, fmt.Errorf("line %d: unknown configuration key %q", lineNumber, key)
		}
		value, err := parseTOMLValue(key, raw, definition.Kind)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		settings[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan TOML: %w", err)
	}
	if schemaVersion != config.SchemaVersion {
		return nil, fmt.Errorf("unsupported schema_version %d", schemaVersion)
	}
	if err := config.ValidateLayer(settings, scope); err != nil {
		return nil, err
	}
	return settings, nil
}

func parseTOMLValue(key, raw string, kind config.Kind) (config.Value, error) {
	if kind == config.String {
		if len(raw) < 2 {
			return config.Value{}, fmt.Errorf("configuration key %q expects a quoted TOML string", key)
		}
		var decoded string
		var err error
		switch raw[0] {
		case '"':
			if err = validateBasicString(raw); err == nil {
				decoded, err = strconv.Unquote(raw)
			}
		case '\'':
			if raw[len(raw)-1] != '\'' || strings.ContainsRune(raw[1:len(raw)-1], '\'') {
				err = errors.New("unterminated literal string")
			} else {
				decoded = raw[1 : len(raw)-1]
			}
		default:
			err = errors.New("value is not quoted")
		}
		if err != nil {
			return config.Value{}, fmt.Errorf("configuration key %q expects a valid TOML string: %v", key, err)
		}
		return config.ParseValue(key, decoded)
	}
	return config.ParseValue(key, raw)
}

func validateBasicString(raw string) error {
	if len(raw) < 2 || raw[len(raw)-1] != '"' {
		return errors.New("unterminated basic string")
	}
	for index := 1; index < len(raw)-1; index++ {
		if raw[index] != '\\' {
			continue
		}
		index++
		if index >= len(raw)-1 {
			return errors.New("unterminated escape")
		}
		switch raw[index] {
		case '"', '\\', 'b', 't', 'n', 'f', 'r':
		case 'u':
			index += 4
			if index >= len(raw) {
				return errors.New("invalid Unicode escape")
			}
		case 'U':
			index += 8
			if index >= len(raw) {
				return errors.New("invalid Unicode escape")
			}
		default:
			return fmt.Errorf("unsupported escape \\%c", raw[index])
		}
	}
	return nil
}

func encode(settings config.Settings) []byte {
	var builder strings.Builder
	fmt.Fprintf(&builder, "schema_version = %d\n", config.SchemaVersion)
	currentSection := ""
	for _, key := range config.Keys() {
		value, ok := settings[key]
		if !ok {
			continue
		}
		section, name, _ := strings.Cut(key, ".")
		if section != currentSection {
			fmt.Fprintf(&builder, "\n[%s]\n", section)
			currentSection = section
		}
		fmt.Fprintf(&builder, "%s = %s\n", name, encodeValue(value))
	}
	return []byte(builder.String())
}

func encodeValue(value config.Value) string {
	if text, ok := value.StringField(); ok {
		return strconv.Quote(text)
	}
	return value.String()
}

func withoutComment(line string) string {
	if index := commentIndex(line); index >= 0 {
		return line[:index]
	}
	return line
}

func commentIndex(line string) int {
	var quote rune
	escaped := false
	for index, character := range line {
		if escaped {
			escaped = false
			continue
		}
		if quote == '"' && character == '\\' {
			escaped = true
			continue
		}
		if character == '"' || character == '\'' {
			if quote == 0 {
				quote = character
			} else if quote == character {
				quote = 0
			}
			continue
		}
		if character == '#' && quote == 0 {
			return index
		}
	}
	return -1
}

func updateDocument(encoded []byte, key string, value config.Value) []byte {
	text := strings.ReplaceAll(string(encoded), "\r\n", "\n")
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	targetSection, targetName, _ := strings.Cut(key, ".")
	section := ""
	sectionEnd := -1

	for index, line := range lines {
		content := strings.TrimSpace(withoutComment(line))
		if strings.HasPrefix(content, "[") && strings.HasSuffix(content, "]") {
			if section == targetSection && sectionEnd == -1 {
				sectionEnd = index
			}
			section = strings.TrimSpace(content[1 : len(content)-1])
			continue
		}
		if section != targetSection {
			continue
		}
		equals := strings.IndexByte(content, '=')
		if equals < 0 || strings.TrimSpace(content[:equals]) != targetName {
			continue
		}

		originalEquals := strings.IndexByte(line, '=')
		updated := line[:originalEquals+1] + " " + encodeValue(value)
		if comment := commentIndex(line); comment >= 0 {
			updated += " " + strings.TrimLeft(line[comment:], " \t")
		}
		lines[index] = updated
		return []byte(strings.Join(lines, "\n") + "\n")
	}

	assignment := targetName + " = " + encodeValue(value)
	if sectionEnd >= 0 {
		lines = append(lines[:sectionEnd], append([]string{assignment, ""}, lines[sectionEnd:]...)...)
	} else if section == targetSection {
		lines = append(lines, assignment)
	} else {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, "["+targetSection+"]", assignment)
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

var _ config.Store = (*Store)(nil)
