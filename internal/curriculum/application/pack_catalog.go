package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

const PackCatalogAlgorithmVersionV1 = "pack-catalog-v1"

type PackCatalogV1 struct {
	source        PackCatalogSource
	cache         PackCatalogCache
	currentKelyro string
}

func NewPackCatalogV1(source PackCatalogSource, cache PackCatalogCache, currentKelyroVersion string) *PackCatalogV1 {
	return &PackCatalogV1{source: source, cache: cache, currentKelyro: strings.TrimPrefix(strings.TrimSpace(currentKelyroVersion), "v")}
}

func (service *PackCatalogV1) Catalog(ctx context.Context) (PackCatalogView, error) {
	const operation = "load Learning Pack catalog"
	if service == nil || service.cache == nil {
		return PackCatalogView{}, Classify(ErrorUnavailable, operation, fmt.Errorf("pack catalog cache is not configured"))
	}
	if err := ctx.Err(); err != nil {
		return PackCatalogView{}, Classify(ErrorUnavailable, operation, err)
	}
	var snapshot curriculum.PackCatalogSnapshot
	view := PackCatalogView{AlgorithmVersion: PackCatalogAlgorithmVersionV1}
	if service.source != nil {
		fresh, sourceErr := service.source.Load(ctx)
		if sourceErr == nil {
			var normalized curriculum.PackCatalogSnapshot
			normalized, sourceErr = service.normalize(fresh)
			if sourceErr == nil {
				if err := service.cache.Replace(ctx, normalized); err != nil {
					return PackCatalogView{}, RepositoryError(operation, err)
				}
				view.Snapshot = normalized
				return view, nil
			}
		}
		view.Offline = true
		view.SourceWarning = sourceErr.Error()
	}
	cached, err := service.cache.Load(ctx)
	if err != nil {
		return PackCatalogView{}, RepositoryError(operation, err)
	}
	snapshot, err = service.normalize(cached)
	if err != nil {
		return PackCatalogView{}, Invalid(operation, err)
	}
	view.Snapshot = snapshot
	view.Offline = true
	return view, nil
}

func (service *PackCatalogV1) Search(ctx context.Context, query string) (PackCatalogView, error) {
	const operation = "search Learning Pack catalog"
	query = normalizeCatalogText(query)
	if query == "" {
		return PackCatalogView{}, Invalid(operation, fmt.Errorf("catalog query is empty"))
	}
	view, err := service.Catalog(ctx)
	if err != nil {
		return PackCatalogView{}, err
	}
	tokens := strings.Fields(query)
	type ranked struct {
		entry curriculum.PackCatalogEntry
		rank  int
	}
	var matches []ranked
	for _, entry := range view.Snapshot.Entries {
		haystack := normalizeCatalogText(strings.Join([]string{entry.PackID.String(), entry.Name, entry.Description, entry.Domain, entry.Target, entry.Maintainer, entry.Source.Name}, " "))
		matched := true
		for _, token := range tokens {
			if !strings.Contains(haystack, token) {
				matched = false
				break
			}
		}
		if !matched {
			continue
		}
		rank := 3
		if query == normalizeCatalogText(entry.PackID.String()) || query == normalizeCatalogText(entry.Name) {
			rank = 0
		} else if strings.HasPrefix(normalizeCatalogText(entry.PackID.String()), query) || strings.HasPrefix(normalizeCatalogText(entry.Name), query) {
			rank = 1
		} else if strings.Contains(normalizeCatalogText(entry.PackID.String()+" "+entry.Name), query) {
			rank = 2
		}
		matches = append(matches, ranked{entry: entry, rank: rank})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].rank != matches[j].rank {
			return matches[i].rank < matches[j].rank
		}
		return matches[i].entry.PackID.String() < matches[j].entry.PackID.String()
	})
	view.Snapshot.Entries = make([]curriculum.PackCatalogEntry, len(matches))
	for index, match := range matches {
		view.Snapshot.Entries[index] = match.entry
	}
	return view, nil
}

func (service *PackCatalogV1) normalize(snapshot curriculum.PackCatalogSnapshot) (curriculum.PackCatalogSnapshot, error) {
	snapshot = curriculum.SortPackCatalog(snapshot)
	current, currentErr := curriculum.NewPackVersion(service.currentKelyro)
	for entryIndex := range snapshot.Entries {
		for versionIndex := range snapshot.Entries[entryIndex].Versions {
			version := &snapshot.Entries[entryIndex].Versions[versionIndex]
			version.Compatibility = curriculum.PackCompatibilityUnknown
			if currentErr == nil {
				version.Compatibility = curriculum.PackCompatible
				if compareSemver(parseSemver(current.String()), parseSemver(version.MinimumKelyroVersion.String())) < 0 {
					version.Compatibility = curriculum.PackIncompatible
				}
			}
		}
	}
	if err := snapshot.Validate(); err != nil {
		return curriculum.PackCatalogSnapshot{}, err
	}
	return snapshot, nil
}

func normalizeCatalogText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

var _ PackCatalogService = (*PackCatalogV1)(nil)
