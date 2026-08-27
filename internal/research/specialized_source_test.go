package research

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestSpecializedTechnicalSourcesAreDomainGeneral(t *testing.T) {
	t.Parallel()
	pythonVersion := mustVersion(t, "3.14")
	playgroundLocator := mustLocator(t, "python/playground")
	playground := Source{
		ID: mustSourceID(t, "python-playground"), Kind: SourcePlayground,
		Locator: playgroundLocator, Version: &pythonVersion, TemporalScope: SourceTemporalVersionBound,
		Metadata: SourceMetadata{Title: "Python runtime playground"}, CreatedAt: mustTimestamp(t, 10),
		Specialization: &SourceSpecialization{
			Kind: SourcePlayground, AlgorithmVersion: SpecializedSourceMetadataV1,
			Playground: &PlaygroundDetails{
				Interactive: true, LanguageRuntime: "Python runtime", Version: &pythonVersion,
				Affiliation: SourceAffiliationOfficial, ShareableLocator: mustLocator(t, "python/playground/share/abc"),
			},
		},
	}
	packageVersion := mustVersion(t, "5.4.0")
	packageLocator := mustLocator(t, "typescript/package/reference")
	sourceCodeLocator := mustLocator(t, "typescript/package/source")
	packageReference := Source{
		ID: mustSourceID(t, "typescript-package"), Kind: SourcePackageReference,
		Locator: packageLocator, Version: &packageVersion, TemporalScope: SourceTemporalVersionBound,
		Metadata: SourceMetadata{Title: "Portable client reference"}, CreatedAt: mustTimestamp(t, 10),
		Specialization: &SourceSpecialization{
			Kind: SourcePackageReference, AlgorithmVersion: SpecializedSourceMetadataV1,
			PackageReference: &PackageReferenceDetails{
				PackageModule: "@example/portable-client", Symbol: "Client.connect",
				Version: &packageVersion, CanonicalDocsLocator: packageLocator, SourceCodeLocator: &sourceCodeLocator,
			},
		},
	}
	revision := mustVersion(t, "2022")
	standardLocator := mustLocator(t, "standards/rfc-9110")
	standard := Source{
		ID: mustSourceID(t, "http-standard"), Kind: SourceStandard,
		Locator: standardLocator, Version: &revision, TemporalScope: SourceTemporalCurrent,
		Metadata: SourceMetadata{Title: "HTTP Semantics"}, CreatedAt: mustTimestamp(t, 10),
		Specialization: &SourceSpecialization{
			Kind: SourceStandard, AlgorithmVersion: SpecializedSourceMetadataV1,
			Standard: &StandardDetails{
				StandardsBody: "IETF", StandardID: "RFC 9110", Revision: &revision,
				Status: StandardActive, OfficialLocator: standardLocator,
			},
		},
	}

	for _, source := range []Source{playground, packageReference, standard} {
		if err := source.Validate(); err != nil {
			t.Errorf("Source(%s).Validate() error = %v", source.ID, err)
		}
		encoded, err := EncodeSourceSpecialization(*source.Specialization)
		if err != nil {
			t.Fatalf("EncodeSourceSpecialization(%s) error = %v", source.ID, err)
		}
		parsed, err := ParseSourceSpecialization(encoded)
		if err != nil || !reflect.DeepEqual(parsed, *source.Specialization) {
			t.Fatalf("specialization roundtrip for %s = (%+v, %v)", source.ID, parsed, err)
		}
	}
}

func TestSpecializedSourceValidationRejectsMismatchedOrIncompleteMetadata(t *testing.T) {
	t.Parallel()
	version := mustVersion(t, "2")
	otherVersion := mustVersion(t, "3")
	locator := mustLocator(t, "specialized")
	base := Source{
		ID: mustSourceID(t, "specialized"), Kind: SourcePlayground,
		Locator: locator, Version: &version, TemporalScope: SourceTemporalVersionBound,
		Metadata: SourceMetadata{Title: "Interactive runtime"}, CreatedAt: mustTimestamp(t, 10),
	}
	if err := base.Validate(); err == nil || !strings.Contains(err.Error(), "requires specialized") {
		t.Fatalf("playground without details error = %v", err)
	}

	valid := SourceSpecialization{
		Kind: SourcePlayground, AlgorithmVersion: SpecializedSourceMetadataV1,
		Playground: &PlaygroundDetails{
			Interactive: true, LanguageRuntime: "Ruby runtime", Version: &version,
			Affiliation: SourceAffiliationCommunity, ShareableLocator: mustLocator(t, "specialized/share"),
		},
	}
	tests := []struct {
		name   string
		mutate func(Source, SourceSpecialization) Source
		needle string
	}{
		{"not interactive", func(source Source, specialization SourceSpecialization) Source {
			specialization.Playground.Interactive = false
			source.Specialization = &specialization
			return source
		}, "interactive"},
		{"version mismatch", func(source Source, specialization SourceSpecialization) Source {
			specialization.Playground.Version = &otherVersion
			source.Specialization = &specialization
			return source
		}, "do not match"},
		{"wrong union kind", func(source Source, specialization SourceSpecialization) Source {
			specialization.Kind = SourceStandard
			source.Specialization = &specialization
			return source
		}, "standard details"},
		{"unsupported owner", func(source Source, specialization SourceSpecialization) Source {
			source.Kind = SourceOther
			source.Specialization = &specialization
			return source
		}, "does not match"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := test.mutate(base, *valid.Clone())
			if err := candidate.Validate(); err == nil || !strings.Contains(err.Error(), test.needle) {
				t.Fatalf("Validate() error = %v, want %q", err, test.needle)
			}
		})
	}
}

func TestPackageAndStandardSpecializationRequireCanonicalLocators(t *testing.T) {
	t.Parallel()
	version := mustVersion(t, "2026")
	canonical := mustLocator(t, "canonical")
	other := mustLocator(t, "other")
	packageSource := Source{
		ID: mustSourceID(t, "package-canonical"), Kind: SourcePackageReference,
		Locator: canonical, Version: &version, TemporalScope: SourceTemporalCurrent,
		Metadata: SourceMetadata{Title: "Package reference"}, CreatedAt: mustTimestamp(t, 10),
		Specialization: &SourceSpecialization{
			Kind: SourcePackageReference, AlgorithmVersion: SpecializedSourceMetadataV1,
			PackageReference: &PackageReferenceDetails{PackageModule: "portable.module", Version: &version, CanonicalDocsLocator: other},
		},
	}
	if err := packageSource.Validate(); err == nil || !strings.Contains(err.Error(), "canonical docs") {
		t.Fatalf("package locator error = %v", err)
	}
	standard := packageSource
	standard.Kind = SourceStandard
	standard.Specialization = &SourceSpecialization{
		Kind: SourceStandard, AlgorithmVersion: SpecializedSourceMetadataV1,
		Standard: &StandardDetails{StandardsBody: "W3C", StandardID: "REC-example", Revision: &version, Status: StandardActive, OfficialLocator: other},
	}
	if err := standard.Validate(); err == nil || !strings.Contains(err.Error(), "official locator") {
		t.Fatalf("standard locator error = %v", err)
	}
}

func TestSpecializedSourceJSONRejectsUnknownNonCanonicalAndOversizedInput(t *testing.T) {
	t.Parallel()
	locator := mustLocator(t, "playground/share")
	specialization := SourceSpecialization{
		Kind: SourcePlayground, AlgorithmVersion: SpecializedSourceMetadataV1,
		Playground: &PlaygroundDetails{
			Interactive: true, LanguageRuntime: "JVM", Affiliation: SourceAffiliationOfficial, ShareableLocator: locator,
		},
	}
	encoded, err := EncodeSourceSpecialization(specialization)
	if err != nil {
		t.Fatal(err)
	}
	pretty := append([]byte("\n"), encoded...)
	unknown := bytes.Replace(encoded, []byte(`"algorithm_version"`), []byte(`"unknown":true,"algorithm_version"`), 1)
	for name, input := range map[string][]byte{
		"non-canonical": pretty,
		"unknown field": unknown,
		"oversized":     bytes.Repeat([]byte("x"), MaximumSpecializedSourceJSONBytes+1),
	} {
		if _, err := ParseSourceSpecialization(input); err == nil {
			t.Fatalf("ParseSourceSpecialization accepted %s input", name)
		}
	}
}

func TestSpecializedSourceClosedVocabularyRejectsUnknownValues(t *testing.T) {
	t.Parallel()
	for _, affiliation := range []SourceAffiliation{SourceAffiliationOfficial, SourceAffiliationCommunity} {
		if err := affiliation.Validate(); err != nil {
			t.Errorf("SourceAffiliation(%q).Validate() error = %v", affiliation, err)
		}
	}
	for _, status := range []StandardStatus{
		StandardDraft, StandardActive, StandardSuperseded,
		StandardWithdrawn, StandardDeprecated, StandardUnknown,
	} {
		if err := status.Validate(); err != nil {
			t.Errorf("StandardStatus(%q).Validate() error = %v", status, err)
		}
	}
	if err := SourceAffiliation("partner").Validate(); err == nil {
		t.Fatal("SourceAffiliation.Validate() accepted unknown value")
	}
	if err := StandardStatus("published").Validate(); err == nil {
		t.Fatal("StandardStatus.Validate() accepted unknown value")
	}
}
