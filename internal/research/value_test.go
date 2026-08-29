package research

import (
	"math"
	"testing"
	"time"
)

const domainFixtureVersion = "i03-step01-v1"

func TestIdentifiersRejectEmptyPaddedAndWhitespaceValues(t *testing.T) {
	t.Parallel()

	constructors := []struct {
		name string
		new  func(string) error
	}{
		{"id", func(value string) error { _, err := NewID(value); return err }},
		{"source id", func(value string) error { _, err := NewSourceID(value); return err }},
		{"claim id", func(value string) error { _, err := NewClaimID(value); return err }},
	}
	for _, constructor := range constructors {
		constructor := constructor
		t.Run(constructor.name, func(t *testing.T) {
			t.Parallel()
			if err := constructor.new(domainFixtureVersion + ".valid"); err != nil {
				t.Fatalf("valid identifier error = %v", err)
			}
			for _, invalid := range []string{"", "   ", " padded", "padded ", "two words", "line\nbreak"} {
				if err := constructor.new(invalid); err == nil {
					t.Errorf("constructor accepted %q", invalid)
				}
			}
		})
	}
}

func TestSourceLocatorValidatesAndNormalizesAbsoluteHTTPURLs(t *testing.T) {
	t.Parallel()

	locator, err := NewSourceLocator("HTTPS://GO.DEV/ref/spec#Types")
	if err != nil {
		t.Fatalf("NewSourceLocator() error = %v", err)
	}
	if got := locator.String(); got != "https://go.dev/ref/spec#Types" {
		t.Fatalf("normalized locator = %q", got)
	}
	for _, invalid := range []string{
		"", "go.dev/doc", "ftp://go.dev/doc", "https:///missing-host",
		"https://user:secret@example.com/doc", "https://example.com/white space",
		"https://example.com/doc?access_token=secret", "https://example.com/doc?x=1;y=2",
		"https://example.com/doc#access_token=secret",
		"https://example.com/doc\x00tail", "https://example.com\\@127.0.0.1/doc",
		"https://example.com/" + string(make([]byte, maximumSourceLocatorBytes)),
	} {
		if _, err := NewSourceLocator(invalid); err == nil {
			t.Errorf("NewSourceLocator(%q) accepted invalid locator", invalid)
		}
	}
}

func TestConfidenceAndFreshnessScoresUseClosedUnitInterval(t *testing.T) {
	t.Parallel()

	for _, value := range []float64{0, 0.5, 1} {
		confidence, err := NewClaimConfidence(value)
		if err != nil || confidence.Value() != value {
			t.Errorf("NewClaimConfidence(%v) = %v, %v", value, confidence.Value(), err)
		}
		score, err := NewFreshnessScore(value)
		if err != nil || score.Value() != value {
			t.Errorf("NewFreshnessScore(%v) = %v, %v", value, score.Value(), err)
		}
	}
	for _, value := range []float64{-0.01, 1.01, math.NaN(), math.Inf(1)} {
		if _, err := NewClaimConfidence(value); err == nil {
			t.Errorf("NewClaimConfidence(%v) accepted invalid score", value)
		}
		if _, err := NewFreshnessScore(value); err == nil {
			t.Errorf("NewFreshnessScore(%v) accepted invalid score", value)
		}
	}
}

func TestTimestampNormalizesToUTCAndRejectsInvalidState(t *testing.T) {
	t.Parallel()

	local := time.Date(2026, 8, 24, 9, 0, 0, 0, time.FixedZone("PET", -5*60*60))
	timestamp, err := NewTimestamp(local)
	if err != nil {
		t.Fatalf("NewTimestamp() error = %v", err)
	}
	if timestamp.Time().Location() != time.UTC || timestamp.Time().Hour() != 14 {
		t.Fatalf("normalized timestamp = %v", timestamp.Time())
	}
	if _, err := NewTimestamp(time.Time{}); err == nil {
		t.Fatal("NewTimestamp() accepted zero time")
	}
	invalid := Timestamp{value: local}
	if err := invalid.Validate(); err == nil {
		t.Fatal("Timestamp.Validate() accepted non-UTC internal state")
	}
}

func TestResearchTopicAndPurposeRemainDomainGeneral(t *testing.T) {
	t.Parallel()

	topic, err := NewResearchTopic("Bayesian inference", "statistics", "")
	if err != nil {
		t.Fatalf("NewResearchTopic() error = %v", err)
	}
	if topic.Technology != "" {
		t.Fatalf("technology = %q, want optional empty value", topic.Technology)
	}
	if _, err := NewResearchTopic(" ", "statistics", ""); err == nil {
		t.Fatal("NewResearchTopic() accepted empty subject")
	}
	if err := ResearchPurpose("invented").Validate(); err == nil {
		t.Fatal("ResearchPurpose.Validate() accepted unknown purpose")
	}
}

func mustID(t *testing.T, suffix string) ID {
	t.Helper()
	id, err := NewID(domainFixtureVersion + "." + suffix)
	if err != nil {
		t.Fatalf("NewID() error = %v", err)
	}
	return id
}

func mustSourceID(t *testing.T, suffix string) SourceID {
	t.Helper()
	id, err := NewSourceID(domainFixtureVersion + ".source." + suffix)
	if err != nil {
		t.Fatalf("NewSourceID() error = %v", err)
	}
	return id
}

func mustClaimID(t *testing.T, suffix string) ClaimID {
	t.Helper()
	id, err := NewClaimID(domainFixtureVersion + ".claim." + suffix)
	if err != nil {
		t.Fatalf("NewClaimID() error = %v", err)
	}
	return id
}

func mustTimestamp(t *testing.T, hour int) Timestamp {
	t.Helper()
	timestamp, err := NewTimestamp(time.Date(2026, 8, 24, hour, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewTimestamp() error = %v", err)
	}
	return timestamp
}

func mustLocator(t *testing.T, path string) SourceLocator {
	t.Helper()
	locator, err := NewSourceLocator("https://example.test/" + path)
	if err != nil {
		t.Fatalf("NewSourceLocator() error = %v", err)
	}
	return locator
}

func mustVersion(t *testing.T, value string) SourceVersion {
	t.Helper()
	version, err := NewSourceVersion(value)
	if err != nil {
		t.Fatalf("NewSourceVersion() error = %v", err)
	}
	return version
}

func mustConfidence(t *testing.T, value float64) ClaimConfidence {
	t.Helper()
	confidence, err := NewClaimConfidence(value)
	if err != nil {
		t.Fatalf("NewClaimConfidence() error = %v", err)
	}
	return confidence
}
