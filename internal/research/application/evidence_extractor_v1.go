package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mishaaac/kelyro/internal/research"
)

type deterministicEvidenceExtractorV1 struct{}

func NewDeterministicEvidenceExtractorV1() EvidenceExtractor {
	return deterministicEvidenceExtractorV1{}
}

func (deterministicEvidenceExtractorV1) Extract(ctx context.Context, request EvidenceExtractionRequest) (EvidenceExtractionResult, error) {
	const operation = "extract deterministic evidence candidates"
	if ctx == nil {
		return EvidenceExtractionResult{}, invalid(operation, errors.New("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return EvidenceExtractionResult{}, Classify(ErrorUnavailable, operation, err)
	}
	if err := request.Validate(); err != nil {
		return EvidenceExtractionResult{}, invalid(operation, err)
	}

	documentTopic := textHasTopicAnchor(request.Source.Title, request)
	if !documentTopic {
		for _, heading := range request.Source.Headings {
			if textHasTopicAnchor(heading.Text, request) {
				documentTopic = true
				break
			}
		}
	}

	candidates := make([]EvidenceCandidate, 0, len(request.Source.Headings)+len(request.Source.TextSegments)+len(request.Source.VersionHints)+1)
	if request.Source.Title != "" {
		candidate, admitted := buildEvidenceCandidate(request, EvidenceCandidateMetadata, "metadata/title", "title: "+request.Source.Title, "", "", documentTopic)
		if admitted {
			candidates = append(candidates, candidate)
		}
	}
	for index, heading := range request.Source.Headings {
		if err := ctx.Err(); err != nil {
			return EvidenceExtractionResult{}, Classify(ErrorUnavailable, operation, err)
		}
		candidate, admitted := buildEvidenceCandidate(request, EvidenceCandidateHeading, indexedEvidenceLocation("heading", index), heading.Text, "", "", documentTopic)
		if admitted {
			candidates = append(candidates, candidate)
		}
	}
	for index, segment := range request.Source.TextSegments {
		if err := ctx.Err(); err != nil {
			return EvidenceExtractionResult{}, Classify(ErrorUnavailable, operation, err)
		}
		focus := evidenceFocus(segment, request)
		excerpt, before, after := boundedEvidenceWindow(segment, focus)
		if before == "" && index > 0 {
			before = boundedContextEnd(request.Source.TextSegments[index-1])
		}
		if after == "" && index+1 < len(request.Source.TextSegments) {
			after = boundedContextStart(request.Source.TextSegments[index+1])
		}
		candidate, admitted := buildEvidenceCandidate(request, EvidenceCandidatePassage, indexedEvidenceLocation("text", index), excerpt, before, after, documentTopic)
		if admitted {
			candidates = append(candidates, candidate)
		}
	}
	for index, version := range request.Source.VersionHints {
		if err := ctx.Err(); err != nil {
			return EvidenceExtractionResult{}, Classify(ErrorUnavailable, operation, err)
		}
		candidate, admitted := buildEvidenceCandidate(request, EvidenceCandidateMetadata, indexedEvidenceLocation("metadata/version", index), "version: "+version, "", "", documentTopic)
		if admitted {
			candidates = append(candidates, candidate)
		}
	}

	candidates = rankAndDeduplicateEvidenceCandidates(candidates)
	if len(candidates) > MaximumEvidenceCandidatesPerSource {
		candidates = candidates[:MaximumEvidenceCandidatesPerSource]
	}
	result := EvidenceExtractionResult{Candidates: candidates, AlgorithmVersion: EvidenceExtractorV1}
	if err := result.Validate(); err != nil {
		return EvidenceExtractionResult{}, invalid(operation, err)
	}
	return cloneEvidenceExtractionResult(result), nil
}

func buildEvidenceCandidate(
	request EvidenceExtractionRequest,
	kind EvidenceCandidateKind,
	location, excerpt, before, after string,
	documentTopic bool,
) (EvidenceCandidate, bool) {
	score, signals, anchored := scoreEvidenceText(excerpt, kind, request, documentTopic)
	if !anchored || score < MinimumEvidenceCandidateScore {
		return EvidenceCandidate{}, false
	}
	if score > 100 {
		score = 100
	}
	candidate := EvidenceCandidate{
		SourceID: request.Source.SourceID, SnapshotID: request.Snapshot.ID,
		Kind: kind, Location: location, Excerpt: excerpt,
		ExcerptHash:   research.CanonicalEvidenceExcerptHashV1(excerpt),
		ContextBefore: before, ContextAfter: after, Score: score,
		Signals: signals, ExtractorVersion: EvidenceExtractorV1,
	}
	return candidate, candidate.Validate() == nil
}

func scoreEvidenceText(text string, kind EvidenceCandidateKind, request EvidenceExtractionRequest, documentTopic bool) (int, []EvidenceSignal, bool) {
	words := evidenceWords(text)
	wordSet := make(map[string]struct{}, len(words))
	for _, word := range words {
		wordSet[word] = struct{}{}
	}
	score := 0
	signals := make([]EvidenceSignal, 0, MaximumEvidenceCandidateSignals)
	anchored := false
	add := func(signal EvidenceSignal, weight int, anchor bool) {
		for _, existing := range signals {
			if existing == signal {
				return
			}
		}
		signals = append(signals, signal)
		score += weight
		anchored = anchored || anchor
	}

	subjectWords := evidenceWords(request.Topic.Subject)
	if containsEvidencePhrase(words, subjectWords) {
		add(EvidenceSignalTopicExact, 55, true)
	}
	overlap := distinctEvidenceWordOverlap(wordSet, subjectWords)
	if overlap > 0 {
		if overlap > 3 {
			overlap = 3
		}
		add(EvidenceSignalTopicTerms, overlap*12, true)
	}
	if request.Topic.Technology != "" && containsEvidencePhrase(words, evidenceWords(request.Topic.Technology)) {
		add(EvidenceSignalTechnology, 15, true)
	}
	if request.Topic.Domain != "" && containsEvidencePhrase(words, evidenceWords(request.Topic.Domain)) {
		add(EvidenceSignalDomain, 10, true)
	}
	if request.TargetVersion != nil && containsVersionToken(text, request.TargetVersion.String()) {
		add(EvidenceSignalTargetVersion, 25, true)
	}
	if documentTopic && !anchored {
		add(EvidenceSignalDocumentTopic, 20, true)
	}
	if kind == EvidenceCandidateHeading {
		add(EvidenceSignalHeading, 5, false)
	}
	if kind == EvidenceCandidateMetadata {
		add(EvidenceSignalStructuredMetadata, 10, false)
	}
	versionFact := explicitVersionFact(text, kind)
	if versionFact {
		add(EvidenceSignalVersionFact, 20, false)
	}
	releaseFact := explicitReleaseFact(text)
	if releaseFact {
		add(EvidenceSignalReleaseFact, 20, false)
	}
	deprecation := explicitDeprecationMarker(text)
	if deprecation {
		add(EvidenceSignalDeprecationMarker, 35, false)
	}
	if (request.Purpose == research.PurposeReleaseStatus || request.Purpose == research.PurposeVersionBehavior) && (versionFact || releaseFact) {
		add(EvidenceSignalPurposeMatch, 20, false)
	}
	if request.Purpose == research.PurposeDeprecationCheck && deprecation {
		add(EvidenceSignalPurposeMatch, 20, false)
	}
	return score, signals, anchored
}

func textHasTopicAnchor(text string, request EvidenceExtractionRequest) bool {
	words := evidenceWords(text)
	if containsEvidencePhrase(words, evidenceWords(request.Topic.Subject)) {
		return true
	}
	if distinctEvidenceWordOverlap(evidenceWordSet(words), evidenceWords(request.Topic.Subject)) > 0 {
		return true
	}
	if request.Topic.Technology != "" && containsEvidencePhrase(words, evidenceWords(request.Topic.Technology)) {
		return true
	}
	return request.TargetVersion != nil && containsVersionToken(text, request.TargetVersion.String())
}

func evidenceWords(value string) []string {
	var words []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}
	}
	for _, item := range strings.ToLower(value) {
		if unicode.IsLetter(item) || unicode.IsDigit(item) {
			current.WriteRune(item)
		} else {
			flush()
		}
	}
	flush()
	return words
}

func evidenceWordSet(words []string) map[string]struct{} {
	result := make(map[string]struct{}, len(words))
	for _, word := range words {
		result[word] = struct{}{}
	}
	return result
}

func containsEvidencePhrase(text, phrase []string) bool {
	if len(phrase) == 0 || len(phrase) > len(text) {
		return false
	}
	for index := 0; index+len(phrase) <= len(text); index++ {
		matched := true
		for offset := range phrase {
			if text[index+offset] != phrase[offset] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func distinctEvidenceWordOverlap(text map[string]struct{}, subject []string) int {
	seen := make(map[string]struct{}, len(subject))
	count := 0
	for _, word := range subject {
		if _, duplicate := seen[word]; duplicate {
			continue
		}
		seen[word] = struct{}{}
		if _, exists := text[word]; exists {
			count++
		}
	}
	return count
}

func containsVersionToken(text, version string) bool {
	want := strings.ToLower(strings.TrimSpace(version))
	if containsDelimitedEvidenceValue(text, want) {
		return true
	}
	trimmed := strings.TrimPrefix(want, "v")
	return trimmed != want && containsDelimitedEvidenceValue(text, trimmed) ||
		trimmed == want && containsDelimitedEvidenceValue(text, "v"+trimmed)
}

func containsDelimitedEvidenceValue(text, value string) bool {
	if value == "" {
		return false
	}
	lower := strings.ToLower(text)
	for offset := 0; offset <= len(lower)-len(value); {
		index := strings.Index(lower[offset:], value)
		if index < 0 {
			return false
		}
		index += offset
		beforeOK := index == 0 || !isEvidenceWordRuneBefore(lower, index)
		after := index + len(value)
		afterOK := after == len(lower) || !isEvidenceWordRuneAt(lower, after)
		if beforeOK && afterOK {
			return true
		}
		offset = index + 1
	}
	return false
}

func isEvidenceWordRuneBefore(value string, index int) bool {
	item, _ := utf8.DecodeLastRuneInString(value[:index])
	return unicode.IsLetter(item) || unicode.IsDigit(item)
}

func isEvidenceWordRuneAt(value string, index int) bool {
	item, _ := utf8.DecodeRuneInString(value[index:])
	return unicode.IsLetter(item) || unicode.IsDigit(item)
}

func explicitVersionFact(text string, kind EvidenceCandidateKind) bool {
	if kind == EvidenceCandidateMetadata && strings.HasPrefix(strings.ToLower(text), "version: ") {
		return strings.TrimSpace(text[len("version: "):]) != ""
	}
	words := evidenceWordSet(evidenceWords(text))
	_, version := words["version"]
	_, since := words["since"]
	_, introduced := words["introduced"]
	return containsEvidenceDigit(text) && (version || since || introduced)
}

func explicitReleaseFact(text string) bool {
	words := evidenceWordSet(evidenceWords(text))
	for _, marker := range []string{"release", "released", "stable", "preview", "available", "lanzamiento", "publicada", "publicado", "disponible"} {
		if _, exists := words[marker]; exists && containsEvidenceDigit(text) {
			return true
		}
	}
	return false
}

func containsEvidenceDigit(value string) bool {
	return strings.IndexFunc(value, unicode.IsDigit) >= 0
}

func explicitDeprecationMarker(text string) bool {
	words := evidenceWordSet(evidenceWords(text))
	for _, marker := range []string{"deprecated", "deprecation", "obsolete", "obsoleto", "obsoleta", "deprecado", "deprecada"} {
		if _, exists := words[marker]; exists {
			return true
		}
	}
	normalized := strings.Join(evidenceWords(text), " ")
	for _, phrase := range []string{"no longer supported", "removed in", "en desuso", "ya no es compatible", "eliminado en", "eliminada en"} {
		if strings.Contains(normalized, phrase) {
			return true
		}
	}
	return false
}

func evidenceFocus(text string, request EvidenceExtractionRequest) int {
	lower := strings.ToLower(text)
	for _, value := range []string{request.Topic.Subject, request.Topic.Technology, request.Topic.Domain} {
		if value != "" {
			if index := strings.Index(lower, strings.ToLower(value)); index >= 0 {
				return index
			}
		}
	}
	if request.TargetVersion != nil {
		if index := strings.Index(lower, strings.ToLower(request.TargetVersion.String())); index >= 0 {
			return index
		}
	}
	for _, marker := range []string{"deprecated", "deprecation", "release", "version", "obsolete", "deprecado", "lanzamiento"} {
		if index := strings.Index(lower, marker); index >= 0 {
			return index
		}
	}
	return 0
}

func boundedEvidenceWindow(value string, focus int) (string, string, string) {
	if len(value) <= MaximumEvidenceCandidateExcerptBytes {
		return value, "", ""
	}
	if focus < 0 || focus >= len(value) {
		focus = 0
	}
	start := focus - MaximumEvidenceCandidateExcerptBytes/3
	if start < 0 {
		start = 0
	}
	if maximumStart := len(value) - MaximumEvidenceCandidateExcerptBytes; start > maximumStart {
		start = maximumStart
	}
	start = utf8BoundaryForward(value, start)
	end := start + MaximumEvidenceCandidateExcerptBytes
	if end > len(value) {
		end = len(value)
	}
	end = utf8BoundaryBackward(value, end)
	if start > 0 {
		if offset := strings.IndexByte(value[start:end], ' '); offset >= 0 && start+offset < focus {
			start += offset + 1
		}
	}
	if end < len(value) {
		if offset := strings.LastIndexByte(value[start:end], ' '); offset > 0 && start+offset > focus {
			end = start + offset
		}
	}
	excerpt := strings.TrimSpace(value[start:end])
	return excerpt, boundedContextEnd(value[:start]), boundedContextStart(value[end:])
}

func boundedContextStart(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= MaximumEvidenceCandidateContextBytes {
		return value
	}
	end := utf8BoundaryBackward(value, MaximumEvidenceCandidateContextBytes)
	if offset := strings.LastIndexByte(value[:end], ' '); offset > 0 {
		end = offset
	}
	return strings.TrimSpace(value[:end])
}

func boundedContextEnd(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= MaximumEvidenceCandidateContextBytes {
		return value
	}
	start := utf8BoundaryForward(value, len(value)-MaximumEvidenceCandidateContextBytes)
	if offset := strings.IndexByte(value[start:], ' '); offset >= 0 {
		start += offset + 1
	}
	return strings.TrimSpace(value[start:])
}

func utf8BoundaryForward(value string, index int) int {
	for index < len(value) && !utf8.RuneStart(value[index]) {
		index++
	}
	return index
}

func utf8BoundaryBackward(value string, index int) int {
	if index > len(value) {
		index = len(value)
	}
	for index > 0 && index < len(value) && !utf8.RuneStart(value[index]) {
		index--
	}
	return index
}

func indexedEvidenceLocation(prefix string, index int) string {
	return fmt.Sprintf("%s[%04d]", prefix, index)
}

func rankAndDeduplicateEvidenceCandidates(candidates []EvidenceCandidate) []EvidenceCandidate {
	sort.SliceStable(candidates, func(left, right int) bool {
		if candidates[left].Score != candidates[right].Score {
			return candidates[left].Score > candidates[right].Score
		}
		if candidates[left].Kind != candidates[right].Kind {
			return candidates[left].Kind < candidates[right].Kind
		}
		if candidates[left].Location != candidates[right].Location {
			return candidates[left].Location < candidates[right].Location
		}
		return candidates[left].ExcerptHash < candidates[right].ExcerptHash
	})
	result := make([]EvidenceCandidate, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if _, duplicate := seen[candidate.ExcerptHash]; duplicate {
			continue
		}
		seen[candidate.ExcerptHash] = struct{}{}
		result = append(result, cloneEvidenceCandidate(candidate))
	}
	return result
}

func cloneEvidenceCandidate(candidate EvidenceCandidate) EvidenceCandidate {
	clone := candidate
	clone.Signals = append([]EvidenceSignal(nil), candidate.Signals...)
	return clone
}

func cloneEvidenceExtractionResult(result EvidenceExtractionResult) EvidenceExtractionResult {
	clone := result
	clone.Candidates = make([]EvidenceCandidate, len(result.Candidates))
	for index, candidate := range result.Candidates {
		clone.Candidates[index] = cloneEvidenceCandidate(candidate)
	}
	return clone
}

var _ EvidenceExtractor = deterministicEvidenceExtractorV1{}
