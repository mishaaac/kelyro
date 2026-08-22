package learning

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
)

const UnversionedDerivedStateVersion = "unversioned/v0"

type CurriculumSourceKind string

const (
	CurriculumSourceFixture CurriculumSourceKind = "fixture"
	CurriculumSourceImport  CurriculumSourceKind = "import"
	CurriculumSourcePack    CurriculumSourceKind = "pack"
)

func (kind CurriculumSourceKind) Valid() bool {
	switch kind {
	case CurriculumSourceFixture, CurriculumSourceImport, CurriculumSourcePack:
		return true
	default:
		return false
	}
}

type CurriculumInstanceStatus string

const (
	CurriculumInstanceActive    CurriculumInstanceStatus = "active"
	CurriculumInstancePaused    CurriculumInstanceStatus = "paused"
	CurriculumInstanceCompleted CurriculumInstanceStatus = "completed"
	CurriculumInstanceArchived  CurriculumInstanceStatus = "archived"
)

func (status CurriculumInstanceStatus) Valid() bool {
	switch status {
	case CurriculumInstanceActive, CurriculumInstancePaused, CurriculumInstanceCompleted, CurriculumInstanceArchived:
		return true
	default:
		return false
	}
}

// CurriculumInstance binds one immutable curriculum version to one learner and
// learning goal. Progress belongs to this identity, never to the definition.
type CurriculumInstance struct {
	ID         ID
	StudentID  ID
	GoalID     ID
	Curriculum CurriculumRef
	Source     CurriculumSourceKind
	Status     CurriculumInstanceStatus
	CreatedAt  Timestamp
	UpdatedAt  Timestamp
}

func NewCurriculumInstance(id, studentID, goalID ID, curriculum CurriculumRef, source CurriculumSourceKind, createdAt Timestamp) (CurriculumInstance, error) {
	instance := CurriculumInstance{
		ID: id, StudentID: studentID, GoalID: goalID, Curriculum: curriculum,
		Source: source, Status: CurriculumInstanceActive, CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	return instance, instance.Validate()
}

func (instance CurriculumInstance) Validate() error {
	if err := instance.ID.Validate(); err != nil {
		return fmt.Errorf("curriculum instance: %w", err)
	}
	if err := instance.StudentID.Validate(); err != nil {
		return fmt.Errorf("curriculum instance student: %w", err)
	}
	if err := instance.GoalID.Validate(); err != nil {
		return fmt.Errorf("curriculum instance goal: %w", err)
	}
	if err := instance.Curriculum.Validate(); err != nil {
		return err
	}
	if !instance.Source.Valid() {
		return fmt.Errorf("curriculum source kind %q is invalid", instance.Source)
	}
	if !instance.Status.Valid() {
		return fmt.Errorf("curriculum instance status %q is invalid", instance.Status)
	}
	if err := instance.CreatedAt.Validate(); err != nil {
		return fmt.Errorf("curriculum instance created at: %w", err)
	}
	if err := instance.UpdatedAt.Validate(); err != nil {
		return fmt.Errorf("curriculum instance updated at: %w", err)
	}
	if instance.UpdatedAt.Before(instance.CreatedAt) {
		return fmt.Errorf("curriculum instance update precedes creation")
	}
	return nil
}

// InstanceConceptState is learner progress scoped to one curriculum instance.
// Evidence remains in its own append-only aggregate and is never copied here.
type InstanceConceptState struct {
	CurriculumInstanceID     ID
	StudentID                ID
	ConceptID                ID
	Exposure                 ExposureState
	Mastery                  MasteryScore
	MasteryAlgorithmVersion  string
	ProgressionPolicyVersion string
	FirstSeenAt              *Timestamp
	LastSeenAt               *Timestamp
	MasteredAt               *Timestamp
	ReviewDueAt              *Timestamp
	ManualFlags              []string
	UpdatedAt                Timestamp
}

func NewInstanceConceptState(instance CurriculumInstance, conceptID ID, createdAt Timestamp) (InstanceConceptState, error) {
	state := InstanceConceptState{
		CurriculumInstanceID:     instance.ID,
		StudentID:                instance.StudentID,
		ConceptID:                conceptID,
		Exposure:                 ExposureNotSeen,
		MasteryAlgorithmVersion:  UnversionedDerivedStateVersion,
		ProgressionPolicyVersion: UnversionedDerivedStateVersion,
		UpdatedAt:                createdAt,
	}
	return state, state.Validate()
}

func (state InstanceConceptState) Validate() error {
	if err := state.CurriculumInstanceID.Validate(); err != nil {
		return fmt.Errorf("instance concept state curriculum instance: %w", err)
	}
	if err := state.StudentID.Validate(); err != nil {
		return fmt.Errorf("instance concept state student: %w", err)
	}
	if err := state.ConceptID.Validate(); err != nil {
		return fmt.Errorf("instance concept state concept: %w", err)
	}
	if !state.Exposure.Valid() {
		return fmt.Errorf("instance concept exposure state %q is invalid", state.Exposure)
	}
	if err := state.Mastery.Validate(); err != nil {
		return err
	}
	if (state.MasteryAlgorithmVersion == "") != (state.ProgressionPolicyVersion == "") {
		return fmt.Errorf("instance concept derived-state versions must both be present or absent")
	}
	for _, candidate := range []struct {
		name      string
		timestamp *Timestamp
	}{
		{name: "first seen at", timestamp: state.FirstSeenAt},
		{name: "last seen at", timestamp: state.LastSeenAt},
		{name: "mastered at", timestamp: state.MasteredAt},
		{name: "review due at", timestamp: state.ReviewDueAt},
	} {
		if err := validateOptionalTimestamp("instance concept "+candidate.name, candidate.timestamp); err != nil {
			return err
		}
	}
	if err := state.UpdatedAt.Validate(); err != nil {
		return fmt.Errorf("instance concept state updated at: %w", err)
	}
	if (state.FirstSeenAt == nil) != (state.LastSeenAt == nil) {
		return fmt.Errorf("instance concept first and last seen timestamps must both be present or absent")
	}
	if state.Exposure == ExposureNotSeen {
		if state.FirstSeenAt != nil || state.MasteredAt != nil || state.ReviewDueAt != nil {
			return fmt.Errorf("not-seen instance concept cannot have learning timestamps")
		}
	} else if state.FirstSeenAt == nil {
		return fmt.Errorf("seen instance concept requires first and last seen timestamps")
	}
	if state.FirstSeenAt != nil {
		if state.LastSeenAt.Before(*state.FirstSeenAt) {
			return fmt.Errorf("instance concept last seen precedes first seen")
		}
		if state.UpdatedAt.Before(*state.LastSeenAt) {
			return fmt.Errorf("instance concept update precedes last seen")
		}
	}
	if state.MasteredAt != nil {
		if state.FirstSeenAt == nil || state.MasteredAt.Before(*state.FirstSeenAt) || state.LastSeenAt.Before(*state.MasteredAt) {
			return fmt.Errorf("instance concept mastered timestamp is outside seen range")
		}
	}
	if (state.Exposure == ExposureMastered || state.Exposure == ExposureReviewDue) && state.MasteredAt == nil {
		return fmt.Errorf("instance concept exposure %q requires mastered timestamp", state.Exposure)
	}
	if state.Exposure == ExposureReviewDue && state.ReviewDueAt == nil {
		return fmt.Errorf("review-due instance concept requires review due timestamp")
	}
	if state.ReviewDueAt != nil && state.FirstSeenAt == nil {
		return fmt.Errorf("instance concept review cannot be scheduled before first seen")
	}
	if err := validateManualFlags(state.ManualFlags); err != nil {
		return err
	}
	return nil
}

// ConceptState projects instance-scoped progress into the prerequisite engine's
// presentation-neutral snapshot without losing instance ownership in storage.
func (state InstanceConceptState) ConceptState() ConceptState {
	return ConceptState{
		StudentID: state.StudentID, ConceptID: state.ConceptID,
		Exposure: state.Exposure, Mastery: state.Mastery,
		IntroducedAt: cloneTimestamp(state.FirstSeenAt), UpdatedAt: state.UpdatedAt,
	}
}

func validateManualFlags(flags []string) error {
	for index, flag := range flags {
		if _, err := NewID(flag); err != nil {
			return fmt.Errorf("instance concept manual flag: %w", err)
		}
		if index > 0 && flags[index-1] >= flag {
			return fmt.Errorf("instance concept manual flags must be unique and sorted")
		}
	}
	return nil
}

func cloneTimestamp(timestamp *Timestamp) *Timestamp {
	if timestamp == nil {
		return nil
	}
	cloned := *timestamp
	return &cloned
}

// CurriculumFingerprint hashes the complete canonical consumption contract.
// The same definition/version always yields the same fingerprint regardless of
// input node order; changing pedagogical metadata changes the fingerprint.
func CurriculumFingerprint(curriculum Curriculum) (string, error) {
	if err := curriculum.Validate(); err != nil {
		return "", err
	}
	nodes := cloneCurriculumNodes(curriculum.Nodes)
	canonicalizeCurriculumNodes(nodes)
	hasher := sha256.New()
	writeFingerprintString(hasher, curriculum.ContractVersion)
	writeFingerprintString(hasher, curriculum.Reference.ID.String())
	writeFingerprintString(hasher, curriculum.Reference.Version)
	writeFingerprintString(hasher, curriculum.Title)
	writeFingerprintString(hasher, curriculum.Description)
	for _, node := range nodes {
		writeFingerprintString(hasher, node.ID.String())
		writeFingerprintString(hasher, string(node.Type))
		if node.ParentID == nil {
			writeFingerprintString(hasher, "")
		} else {
			writeFingerprintString(hasher, node.ParentID.String())
		}
		writeFingerprintString(hasher, node.Title)
		writeFingerprintString(hasher, node.Description)
		writeFingerprintInt(hasher, int64(node.Order))
		writeFingerprintString(hasher, node.Display.ShortTitle)
		writeFingerprintBool(hasher, node.Display.Hidden)
		writeFingerprintString(hasher, string(node.Status.State))
		writeFingerprintString(hasher, node.Status.Note)
		writeFingerprintString(hasher, node.Version)
		if node.Concept == nil {
			writeFingerprintBool(hasher, false)
			continue
		}
		writeFingerprintBool(hasher, true)
		for _, objective := range node.Concept.Objectives {
			writeFingerprintString(hasher, objective)
		}
		writeFingerprintString(hasher, "objectives:end")
		for _, prerequisite := range node.Concept.Prerequisites {
			writeFingerprintString(hasher, prerequisite.ConceptID.String())
			writeFingerprintString(hasher, string(prerequisite.Requirement))
		}
		writeFingerprintString(hasher, "prerequisites:end")
		writeFingerprintInt(hasher, int64(node.Concept.Difficulty))
		writeFingerprintInt(hasher, int64(node.Concept.EstimatedEffortMinutes))
		writeFingerprintBool(hasher, node.Concept.TheoryRequired)
		for _, expectation := range node.Concept.AssessmentExpectations {
			writeFingerprintString(hasher, expectation)
		}
		writeFingerprintString(hasher, "expectations:end")
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func writeFingerprintString(target hash.Hash, value string) {
	writeFingerprintInt(target, int64(len(value)))
	_, _ = target.Write([]byte(value))
}

func writeFingerprintInt(target hash.Hash, value int64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], uint64(value))
	_, _ = target.Write(encoded[:])
}

func writeFingerprintBool(target hash.Hash, value bool) {
	if value {
		_, _ = target.Write([]byte{1})
		return
	}
	_, _ = target.Write([]byte{0})
}
