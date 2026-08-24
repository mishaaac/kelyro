package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

type researchSourceRegistryRepository struct {
	executor executor
	timeout  time.Duration
}

type registryHintJSON struct {
	SourceKind string `json:"source_kind"`
	Tier       string `json:"tier"`
	Reason     string `json:"reason"`
}

func (repository *researchSourceRegistryRepository) Save(ctx context.Context, entry research.SourceRegistryEntry) error {
	const operation = "save SQLite source registry entry"
	if err := entry.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	domains := make([]string, len(entry.CanonicalDomains))
	for index, domain := range entry.CanonicalDomains {
		domains[index] = domain.String()
	}
	hints := make([]registryHintJSON, len(entry.AuthorityHints))
	for index, hint := range entry.AuthorityHints {
		hints[index] = registryHintJSON{SourceKind: string(hint.SourceKind), Tier: string(hint.Tier), Reason: hint.Reason}
	}
	encodedDomains, err := encodeJSON(operation, domains)
	if err != nil {
		return err
	}
	encodedKinds, err := encodeJSON(operation, sourceKindStrings(entry.SourceKinds))
	if err != nil {
		return err
	}
	encodedHints, err := encodeJSON(operation, hints)
	if err != nil {
		return err
	}
	encodedResearchDomains, err := encodeJSON(operation, entry.ResearchDomains)
	if err != nil {
		return err
	}
	encodedTopics, err := encodeJSON(operation, entry.TopicPatterns)
	if err != nil {
		return err
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	_, err = repository.executor.ExecContext(opCtx, `INSERT INTO source_registry_entries (id,organization,canonical_domains_json,source_kinds_json,authority_hints_json,research_domains_json,topic_patterns_json,notes,status,added_at,last_reviewed_at) VALUES (?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET organization=excluded.organization,canonical_domains_json=excluded.canonical_domains_json,source_kinds_json=excluded.source_kinds_json,authority_hints_json=excluded.authority_hints_json,research_domains_json=excluded.research_domains_json,topic_patterns_json=excluded.topic_patterns_json,notes=excluded.notes,status=excluded.status,added_at=excluded.added_at,last_reviewed_at=excluded.last_reviewed_at`, entry.ID.String(), entry.Organization, encodedDomains, encodedKinds, encodedHints, encodedResearchDomains, encodedTopics, entry.Notes, string(entry.Status), timestampText(entry.AddedAt), timestampText(entry.LastReviewedAt))
	if err != nil {
		if strings.Contains(err.Error(), "duplicate source registry domain") {
			return researchConflict(operation)
		}
		return researchPersistence(operation, err)
	}
	return nil
}

func (repository *researchSourceRegistryRepository) Get(ctx context.Context, id research.ID) (research.SourceRegistryEntry, error) {
	const operation = "get SQLite source registry entry"
	if err := id.Validate(); err != nil {
		return research.SourceRegistryEntry{}, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return research.SourceRegistryEntry{}, err
	}
	defer cancel()
	return scanSourceRegistryEntry(repository.executor.QueryRowContext(opCtx, sourceRegistrySelect+` WHERE id=?`, id.String()), operation)
}

func (repository *researchSourceRegistryRepository) List(ctx context.Context) ([]research.SourceRegistryEntry, error) {
	const operation = "list SQLite source registry entries"
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return nil, err
	}
	defer cancel()
	rows, err := repository.executor.QueryContext(opCtx, sourceRegistrySelect+` ORDER BY id`)
	if err != nil {
		return nil, researchPersistence(operation, err)
	}
	defer rows.Close()
	result := make([]research.SourceRegistryEntry, 0)
	for rows.Next() {
		entry, scanErr := scanSourceRegistryEntry(rows, operation)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, researchPersistence(operation, err)
	}
	return result, nil
}

const sourceRegistrySelect = `SELECT id,organization,canonical_domains_json,source_kinds_json,authority_hints_json,research_domains_json,topic_patterns_json,notes,status,added_at,last_reviewed_at FROM source_registry_entries`

func scanSourceRegistryEntry(row rowScanner, operation string) (research.SourceRegistryEntry, error) {
	var idValue, organization, domainsJSON, kindsJSON, hintsJSON, researchDomainsJSON, topicsJSON, notes, status, added, reviewed string
	if err := row.Scan(&idValue, &organization, &domainsJSON, &kindsJSON, &hintsJSON, &researchDomainsJSON, &topicsJSON, &notes, &status, &added, &reviewed); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return research.SourceRegistryEntry{}, researchNotFound(operation)
		}
		return research.SourceRegistryEntry{}, researchPersistence(operation, err)
	}
	id, err := research.NewID(idValue)
	if err != nil {
		return research.SourceRegistryEntry{}, researchPersistence(operation, err)
	}
	var domainValues, kindValues, researchDomains, topics []string
	var hintValues []registryHintJSON
	for _, item := range []struct {
		encoded string
		target  any
	}{{domainsJSON, &domainValues}, {kindsJSON, &kindValues}, {hintsJSON, &hintValues}, {researchDomainsJSON, &researchDomains}, {topicsJSON, &topics}} {
		if err := json.Unmarshal([]byte(item.encoded), item.target); err != nil {
			return research.SourceRegistryEntry{}, researchPersistence(operation, err)
		}
	}
	domains := make([]research.CanonicalDomain, len(domainValues))
	for index, value := range domainValues {
		domain, err := research.NewCanonicalDomain(value)
		if err != nil {
			return research.SourceRegistryEntry{}, researchPersistence(operation, err)
		}
		domains[index] = domain
	}
	kinds := make([]research.SourceKind, len(kindValues))
	for index, value := range kindValues {
		kinds[index] = research.SourceKind(value)
	}
	hints := make([]research.RegistryAuthorityHint, len(hintValues))
	for index, value := range hintValues {
		hints[index] = research.RegistryAuthorityHint{SourceKind: research.SourceKind(value.SourceKind), Tier: research.AuthorityTier(value.Tier), Reason: value.Reason}
	}
	addedAt, err := scanTimestamp(added)
	if err != nil {
		return research.SourceRegistryEntry{}, researchPersistence(operation, err)
	}
	reviewedAt, err := scanTimestamp(reviewed)
	if err != nil {
		return research.SourceRegistryEntry{}, researchPersistence(operation, err)
	}
	entry := research.SourceRegistryEntry{ID: id, Organization: organization, CanonicalDomains: domains, SourceKinds: kinds, AuthorityHints: hints, ResearchDomains: researchDomains, TopicPatterns: topics, Notes: notes, Status: research.RegistryStatus(status), AddedAt: addedAt, LastReviewedAt: reviewedAt}
	if err := entry.Validate(); err != nil {
		return research.SourceRegistryEntry{}, researchPersistence(operation, err)
	}
	return entry, nil
}
