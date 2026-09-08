package application

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mishaaac/kelyro/internal/research"
)

var explicitVersionQualifierV1 = regexp.MustCompile(`(?i)\b(?:version|versi[oó]n)\s+([[:alnum:]][[:alnum:]._-]{0,63})\b`)
var topicVersionTokenV2 = regexp.MustCompile(`(?i)\bv?[0-9]+(?:\.[0-9]+)+\b`)

type deterministicClaimExtractor struct {
	version string
}

func NewDeterministicClaimExtractorV1() ClaimExtractor {
	return deterministicClaimExtractor{version: ClaimExtractorV1}
}

// NewDeterministicClaimExtractorV2 admits literal statements with a
// proportional natural-topic anchor and recognizes version tokens repeated
// verbatim from the topic. It remains conservative about ambiguity and claim
// families and never synthesizes statement text.
func NewDeterministicClaimExtractorV2() ClaimExtractor {
	return deterministicClaimExtractor{version: ClaimExtractorV2}
}

func (extractor deterministicClaimExtractor) Extract(ctx context.Context, request ClaimExtractionRequest) (ClaimExtractionResult, error) {
	const operation = "extract deterministic claim candidates"
	if ctx == nil {
		return ClaimExtractionResult{}, invalid(operation, errors.New("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return ClaimExtractionResult{}, Classify(ErrorUnavailable, operation, err)
	}
	if err := request.Validate(); err != nil {
		return ClaimExtractionResult{}, invalid(operation, err)
	}

	result := ClaimExtractionResult{AlgorithmVersion: extractor.version}
	for index, statement := range splitClaimSentencesV1(request.Evidence.Excerpt) {
		if err := ctx.Err(); err != nil {
			return ClaimExtractionResult{}, Classify(ErrorUnavailable, operation, err)
		}
		candidate, admitted := claimCandidate(request, statement, index, extractor.version)
		if admitted {
			result.Candidates = append(result.Candidates, candidate)
		}
	}
	result.Candidates = deduplicateClaimCandidatesV1(result.Candidates)
	if len(result.Candidates) > MaximumClaimCandidatesPerEvidence {
		result.Candidates = result.Candidates[:MaximumClaimCandidatesPerEvidence]
	}
	if err := result.Validate(); err != nil {
		return ClaimExtractionResult{}, invalid(operation, err)
	}
	return cloneClaimExtractionResult(result), nil
}

func splitClaimSentencesV1(excerpt string) []string {
	var result []string
	start := 0
	for index, item := range excerpt {
		if item != '.' && item != '!' && item != '?' {
			continue
		}
		end := index + utf8.RuneLen(item)
		if end < len(excerpt) {
			next, _ := utf8.DecodeRuneInString(excerpt[end:])
			if !unicode.IsSpace(next) {
				continue
			}
		}
		statement := strings.TrimSpace(excerpt[start:end])
		if statement != "" {
			result = append(result, statement)
		}
		start = end
	}
	// A trailing fragment is deliberately ignored: v1 requires a complete,
	// terminally punctuated sentence instead of guessing where it ends.
	return result
}

func claimCandidate(request ClaimExtractionRequest, statement string, sentenceIndex int, extractorVersion string) (ClaimCandidate, bool) {
	anchored := claimStatementAnchoredV1(statement, request.Topic)
	if extractorVersion == ClaimExtractorV2 {
		anchored = claimStatementAnchoredV2(statement, request.Topic)
	}
	if len(statement) > MaximumClaimCandidateStatementBytes || !utf8.ValidString(statement) ||
		!anchored || claimStatementAmbiguousV1(statement) {
		return ClaimCandidate{}, false
	}
	families := matchedClaimFamilies(statement, extractorVersion == ClaimExtractorV2, request.Topic)
	if len(families) != 1 {
		return ClaimCandidate{}, false
	}
	family := families[0].family
	version, valid := explicitClaimVersion(statement, request.Topic, request.TargetVersion, family, extractorVersion)
	if !valid {
		return ClaimCandidate{}, false
	}
	status, valid := explicitClaimStatusV1(statement)
	if !valid {
		return ClaimCandidate{}, false
	}
	confidence, err := research.NewClaimConfidence(claimFamilyConfidenceV1(family))
	if err != nil {
		panic(err)
	}
	candidate := ClaimCandidate{
		SourceID: request.Evidence.SourceID, SnapshotID: request.Evidence.SnapshotID,
		EvidenceID: request.Evidence.ID, Family: family, Statement: statement,
		StatementHash: CanonicalClaimCandidateStatementHashV1(statement), Scope: request.Topic.Subject,
		VersionScope: version, StatusScope: status, Confidence: confidence,
		ExplicitMarker: families[0].marker, SentenceIndex: sentenceIndex, ExtractorVersion: extractorVersion,
	}
	return candidate, candidate.Validate() == nil
}

type matchedClaimFamilyV1 struct {
	family ClaimFamily
	marker string
}

func matchedClaimFamiliesV1(statement string) []matchedClaimFamilyV1 {
	return matchedClaimFamilies(statement, false, research.ResearchTopic{})
}

func matchedClaimFamilies(statement string, naturalTopic bool, topic research.ResearchTopic) []matchedClaimFamilyV1 {
	words := evidenceWords(statement)
	wordSet := evidenceWordSet(words)
	normalized := strings.Join(words, " ")
	result := make([]matchedClaimFamilyV1, 0, 2)
	add := func(family ClaimFamily, marker string) {
		for _, item := range result {
			if item.family == family {
				return
			}
		}
		result = append(result, matchedClaimFamilyV1{family: family, marker: marker})
	}

	definitionMarkers := []string{" is a ", " is an ", " means ", " refers to ", " es un ", " es una ", " significa ", " se refiere a "}
	if naturalTopic {
		definitionMarkers = append(definitionMarkers, " are named ", " are the ", " defines ", " define ", " provides a way ", " son ", " define ", " definen ")
	}
	if marker := firstClaimPhraseV1(normalized, definitionMarkers); marker != "" &&
		(!naturalTopic || claimDefinitionSubjectPrecedesMarkerV2(normalized, marker, topic)) {
		add(ClaimFamilyExplicitDefinition, strings.TrimSpace(marker))
	}
	if marker := firstClaimWordV1(wordSet, []string{"released", "release", "lanzada", "lanzado", "publicada", "publicado"}); marker != "" &&
		(explicitVersionQualifierV1.MatchString(statement) || naturalTopic && releaseVerbWithTopicVersionV2(normalized, marker, statement, topic)) {
		add(ClaimFamilyVersionReleaseFact, marker)
	}
	if marker := firstClaimWordV1(wordSet, []string{"deprecated", "obsolete", "removed", "deprecado", "deprecada", "obsoleto", "obsoleta", "eliminado", "eliminada"}); marker != "" {
		add(ClaimFamilyDeprecationStatement, marker)
	} else if marker := firstClaimPhraseV1(" "+normalized+" ", []string{" no longer supported ", " ya no es compatible ", " ya no tiene soporte "}); marker != "" {
		add(ClaimFamilyDeprecationStatement, strings.TrimSpace(marker))
	}
	if marker := firstClaimWordV1(wordSet, []string{"available", "supported", "unsupported", "compatible", "incompatible", "disponible", "compatible", "incompatible"}); marker != "" {
		add(ClaimFamilyAvailabilitySupport, marker)
	}
	if marker := firstClaimWordV1(wordSet, []string{"must", "shall", "required", "requires", "debe", "deberá", "requerido", "requerida"}); marker != "" {
		add(ClaimFamilyExplicitRequirement, marker)
	}
	if marker := firstClaimWordV1(wordSet, []string{"should", "recommended", "recommend", "prefer", "preferred", "recomendado", "recomendada", "recomienda", "prefiere"}); marker != "" {
		add(ClaimFamilyExplicitRecommendation, marker)
	}
	return result
}

func claimDefinitionSubjectPrecedesMarkerV2(normalized, marker string, topic research.ResearchTopic) bool {
	index := strings.Index(" "+normalized+" ", marker)
	if index < 0 {
		return false
	}
	prefixWords := evidenceWordSet(evidenceWords((" " + normalized + " ")[:index]))
	for _, word := range distinctEvidenceWordsV2(evidenceWords(topic.Subject)) {
		if _, exists := prefixWords[word]; exists {
			return true
		}
	}
	return false
}

func releaseVerbWithTopicVersionV2(normalized, marker, statement string, topic research.ResearchTopic) bool {
	if !topicVersionRepeatedV2(statement, topic) {
		return false
	}
	if marker != "release" {
		return true
	}
	return strings.Contains(" "+normalized+" ", " to release ")
}

func claimStatementAnchoredV1(statement string, topic research.ResearchTopic) bool {
	words := evidenceWords(statement)
	if containsEvidencePhrase(words, evidenceWords(topic.Subject)) {
		return true
	}
	return topic.Technology != "" && containsEvidencePhrase(words, evidenceWords(topic.Technology))
}

func claimStatementAnchoredV2(statement string, topic research.ResearchTopic) bool {
	statementWords := evidenceWordSet(evidenceWords(statement))
	subjectWords := distinctEvidenceWordsV2(evidenceWords(topic.Subject))
	if len(subjectWords) == 0 {
		return false
	}
	overlap := distinctEvidenceWordOverlap(statementWords, subjectWords)
	required := (len(subjectWords) + 1) / 2
	if required > 3 {
		required = 3
	}
	if overlap < required {
		return false
	}
	for _, word := range subjectWords {
		if _, matched := statementWords[word]; matched && (utf8.RuneCountInString(word) > 2 || strings.IndexFunc(word, unicode.IsDigit) >= 0) {
			return true
		}
	}
	return false
}

func distinctEvidenceWordsV2(words []string) []string {
	result := make([]string, 0, len(words))
	seen := make(map[string]struct{}, len(words))
	for _, word := range words {
		if _, duplicate := seen[word]; duplicate {
			continue
		}
		seen[word] = struct{}{}
		result = append(result, word)
	}
	return result
}

func claimStatementAmbiguousV1(statement string) bool {
	words := evidenceWordSet(evidenceWords(statement))
	for _, marker := range []string{"may", "might", "could", "possibly", "perhaps", "appears", "apparently", "quizá", "quizás", "podría", "parece"} {
		if _, exists := words[marker]; exists {
			return true
		}
	}
	trimmed := strings.TrimSpace(statement)
	return strings.Contains(statement, "http://") || strings.Contains(statement, "https://") ||
		strings.HasPrefix(trimmed, ">") || strings.HasPrefix(trimmed, "\"") || strings.HasPrefix(trimmed, "'") ||
		strings.Contains(trimmed, "`")
}

func explicitClaimVersionV1(statement string, target *research.SourceVersion, family ClaimFamily) (*research.SourceVersion, bool) {
	return explicitClaimVersion(statement, research.ResearchTopic{}, target, family, ClaimExtractorV1)
}

func explicitClaimVersion(statement string, topic research.ResearchTopic, target *research.SourceVersion, family ClaimFamily, extractorVersion string) (*research.SourceVersion, bool) {
	matches := explicitVersionQualifierV1.FindAllStringSubmatch(statement, -1)
	values := make([]string, 0, len(matches)+1)
	for _, match := range matches {
		values = appendUniqueClaimStringV1(values, strings.TrimRight(match[1], ".,;:!?"))
	}
	if target != nil && containsVersionToken(statement, target.String()) {
		values = appendUniqueClaimStringV1(values, target.String())
	}
	if extractorVersion == ClaimExtractorV2 {
		for _, value := range topicVersionTokenV2.FindAllString(topic.Subject, -1) {
			if containsVersionToken(statement, value) {
				values = appendUniqueClaimStringV1(values, strings.TrimPrefix(strings.ToLower(value), "v"))
			}
		}
	}
	if len(values) > 1 {
		return nil, false
	}
	if family == ClaimFamilyVersionReleaseFact && len(values) != 1 {
		return nil, false
	}
	if len(values) == 0 {
		return nil, true
	}
	version, err := research.NewSourceVersion(values[0])
	if err != nil {
		return nil, false
	}
	return &version, true
}

func topicVersionRepeatedV2(statement string, topic research.ResearchTopic) bool {
	for _, value := range topicVersionTokenV2.FindAllString(topic.Subject, -1) {
		if containsVersionToken(statement, value) {
			return true
		}
	}
	return false
}

func explicitClaimStatusV1(statement string) (research.ClaimStatusScope, bool) {
	words := evidenceWordSet(evidenceWords(statement))
	statuses := []struct {
		word   string
		status research.ClaimStatusScope
	}{{"stable", research.ClaimStatusStable}, {"preview", research.ClaimStatusPreview},
		{"experimental", research.ClaimStatusExperimental}, {"legacy", research.ClaimStatusLegacy}}
	selected := research.ClaimStatusAll
	for _, item := range statuses {
		if _, exists := words[item.word]; !exists {
			continue
		}
		if selected != research.ClaimStatusAll && selected != item.status {
			return "", false
		}
		selected = item.status
	}
	return selected, true
}

func claimFamilyConfidenceV1(family ClaimFamily) float64 {
	switch family {
	case ClaimFamilyExplicitDefinition:
		return .85
	case ClaimFamilyVersionReleaseFact, ClaimFamilyDeprecationStatement, ClaimFamilyExplicitRequirement:
		return .90
	case ClaimFamilyAvailabilitySupport:
		return .85
	case ClaimFamilyExplicitRecommendation:
		return .80
	default:
		panic(fmt.Sprintf("unknown claim family %q", family))
	}
}

func firstClaimWordV1(words map[string]struct{}, candidates []string) string {
	for _, candidate := range candidates {
		if _, exists := words[candidate]; exists {
			return candidate
		}
	}
	return ""
}

func firstClaimPhraseV1(value string, candidates []string) string {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return candidate
		}
	}
	return ""
}

func appendUniqueClaimStringV1(values []string, candidate string) []string {
	for _, value := range values {
		if strings.EqualFold(value, candidate) {
			return values
		}
	}
	return append(values, candidate)
}

func deduplicateClaimCandidatesV1(candidates []ClaimCandidate) []ClaimCandidate {
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].SentenceIndex < candidates[j].SentenceIndex
	})
	result := make([]ClaimCandidate, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		key := claimCandidateSemanticKeyV1(candidate)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, candidate)
	}
	return result
}

func claimCandidateSemanticKeyV1(candidate ClaimCandidate) string {
	return string(candidate.Family) + "\x00" + candidate.StatementHash + "\x00" + candidate.Scope + "\x00" +
		optionalSourceVersionString(candidate.VersionScope) + "\x00" + string(candidate.StatusScope)
}

func cloneClaimExtractionResult(result ClaimExtractionResult) ClaimExtractionResult {
	clone := result
	clone.Candidates = make([]ClaimCandidate, len(result.Candidates))
	for index, candidate := range result.Candidates {
		clone.Candidates[index] = cloneClaimCandidate(candidate)
	}
	return clone
}

func cloneClaimCandidate(candidate ClaimCandidate) ClaimCandidate {
	clone := candidate
	clone.VersionScope = cloneSourceVersion(candidate.VersionScope)
	return clone
}

var _ ClaimExtractor = deterministicClaimExtractor{}
