package research

import (
	"fmt"
	"sort"
)

const UpdateScanAlgorithmV1 = "update-scan-v1"

type UpdateSignalType string

const (
	UpdateSignalNewRelease         UpdateSignalType = "new_release"
	UpdateSignalChangedSource      UpdateSignalType = "changed_source"
	UpdateSignalStaleEvidence      UpdateSignalType = "stale_evidence"
	UpdateSignalDeprecatedSubject  UpdateSignalType = "deprecated_subject"
	UpdateSignalUnresolvedConflict UpdateSignalType = "unresolved_conflict"
)

func (signalType UpdateSignalType) Validate() error {
	switch signalType {
	case UpdateSignalNewRelease, UpdateSignalChangedSource, UpdateSignalStaleEvidence,
		UpdateSignalDeprecatedSubject, UpdateSignalUnresolvedConflict:
		return nil
	default:
		return fmt.Errorf("invalid update signal type %q", signalType)
	}
}

func (signalType UpdateSignalType) rank() int {
	switch signalType {
	case UpdateSignalNewRelease:
		return 0
	case UpdateSignalChangedSource:
		return 1
	case UpdateSignalStaleEvidence:
		return 2
	case UpdateSignalDeprecatedSubject:
		return 3
	case UpdateSignalUnresolvedConflict:
		return 4
	default:
		return 5
	}
}

type UpdateSignalOrigin string

const (
	UpdateSignalStoredMetadata UpdateSignalOrigin = "stored_metadata"
	UpdateSignalCurrentLookup  UpdateSignalOrigin = "current_lookup"
)

func (origin UpdateSignalOrigin) Validate() error {
	switch origin {
	case UpdateSignalStoredMetadata, UpdateSignalCurrentLookup:
		return nil
	default:
		return fmt.Errorf("invalid update signal origin %q", origin)
	}
}

// UpdateSignal is bounded change metadata. Detail must summarize the signal;
// it must never contain a fetched page or other unbounded external content.
type UpdateSignal struct {
	Type       UpdateSignalType
	Reference  string
	Detail     string
	Origin     UpdateSignalOrigin
	ObservedAt Timestamp
}

func (signal UpdateSignal) Validate() error {
	if err := signal.Type.Validate(); err != nil {
		return err
	}
	if err := requireText("update signal reference", signal.Reference); err != nil {
		return err
	}
	if err := validateBoundedEvidenceText("update signal detail", signal.Detail, 2<<10, true); err != nil {
		return err
	}
	if err := signal.Origin.Validate(); err != nil {
		return err
	}
	return validateTimestamp("update signal observed at", signal.ObservedAt)
}

type UpdateScanIncompleteReason string

const (
	UpdateScanNetworkDisabled     UpdateScanIncompleteReason = "network_disabled"
	UpdateScanProviderUnavailable UpdateScanIncompleteReason = "provider_unavailable"
	UpdateScanProviderFailed      UpdateScanIncompleteReason = "provider_failed"
)

func (reason UpdateScanIncompleteReason) Validate() error {
	switch reason {
	case UpdateScanNetworkDisabled, UpdateScanProviderUnavailable, UpdateScanProviderFailed:
		return nil
	default:
		return fmt.Errorf("invalid update scan incomplete reason %q", reason)
	}
}

type UpdateScanInventory struct {
	KnownTechnologies int
	KnownReleases     int
	TrackedSources    int
	FreshnessDue      int
}

func (inventory UpdateScanInventory) Validate() error {
	if inventory.KnownTechnologies < 0 || inventory.KnownReleases < 0 ||
		inventory.TrackedSources < 0 || inventory.FreshnessDue < 0 {
		return fmt.Errorf("update scan inventory counts cannot be negative")
	}
	return nil
}

type UpdateScan struct {
	ScannedAt         Timestamp
	Inventory         UpdateScanInventory
	Signals           []UpdateSignal
	IncompleteReasons []UpdateScanIncompleteReason
	AlgorithmVersion  string
}

func (scan UpdateScan) Complete() bool { return len(scan.IncompleteReasons) == 0 }

func (scan UpdateScan) Validate() error {
	if err := validateTimestamp("update scan time", scan.ScannedAt); err != nil {
		return err
	}
	if err := scan.Inventory.Validate(); err != nil {
		return err
	}
	seenSignals := make(map[string]struct{}, len(scan.Signals))
	for index, signal := range scan.Signals {
		if err := signal.Validate(); err != nil {
			return fmt.Errorf("update signal %d: %w", index, err)
		}
		key := string(signal.Type) + "\x00" + signal.Reference
		if _, exists := seenSignals[key]; exists {
			return fmt.Errorf("update scan contains duplicate signal %q", key)
		}
		seenSignals[key] = struct{}{}
	}
	seenReasons := make(map[UpdateScanIncompleteReason]struct{}, len(scan.IncompleteReasons))
	for _, reason := range scan.IncompleteReasons {
		if err := reason.Validate(); err != nil {
			return err
		}
		if _, exists := seenReasons[reason]; exists {
			return fmt.Errorf("update scan contains duplicate incomplete reason %q", reason)
		}
		seenReasons[reason] = struct{}{}
	}
	if scan.AlgorithmVersion != UpdateScanAlgorithmV1 {
		return fmt.Errorf("update scan algorithm must be %q", UpdateScanAlgorithmV1)
	}
	return nil
}

// SortUpdateSignalsV1 produces the stable human/reporting order for scan
// output. A copy is returned so callers retain ownership of their input.
func SortUpdateSignalsV1(signals []UpdateSignal) []UpdateSignal {
	result := append([]UpdateSignal(nil), signals...)
	sort.Slice(result, func(i, j int) bool {
		if result[i].Type.rank() != result[j].Type.rank() {
			return result[i].Type.rank() < result[j].Type.rank()
		}
		if result[i].Reference != result[j].Reference {
			return result[i].Reference < result[j].Reference
		}
		return result[i].ObservedAt.Before(result[j].ObservedAt)
	})
	return result
}
