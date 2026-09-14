package application

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type PackVersioningPolicyV1 struct{}

func NewPackVersioningPolicyV1() PackVersioningPolicyV1 { return PackVersioningPolicyV1{} }

func (PackVersioningPolicyV1) Classify(ctx context.Context, request PackVersioningRequest) (curriculum.PackVersioningDecision, error) {
	const operation = "classify pack version"
	if err := ctx.Err(); err != nil {
		return curriculum.PackVersioningDecision{}, ExternalError(operation, err)
	}
	if err := request.CurrentVersion.Validate(); err != nil {
		return curriculum.PackVersioningDecision{}, Invalid(operation, err)
	}
	if err := request.CandidateVersion.Validate(); err != nil {
		return curriculum.PackVersioningDecision{}, Invalid(operation, err)
	}
	if len(request.Changes) == 0 {
		return curriculum.PackVersioningDecision{}, Invalid(operation, fmt.Errorf("pack versioning requires at least one classified change"))
	}

	decision := curriculum.PackVersioningDecision{
		CurrentVersion: request.CurrentVersion, CandidateVersion: request.CandidateVersion,
		ChangeImpact: curriculum.PackImpactPatch, AlgorithmVersion: curriculum.PackVersioningPolicyVersionV1,
	}
	seen := make(map[curriculum.ID]struct{}, len(request.Changes))
	for _, change := range request.Changes {
		if err := change.Validate(); err != nil {
			return curriculum.PackVersioningDecision{}, Invalid(operation, err)
		}
		if _, exists := seen[change.ID]; exists {
			return curriculum.PackVersioningDecision{}, Invalid(operation, fmt.Errorf("duplicate curriculum change %q", change.ID))
		}
		seen[change.ID] = struct{}{}
		classification := classifyPackChange(change)
		decision.Classifications = append(decision.Classifications, classification)
		if packImpactRank(classification.Impact) > packImpactRank(decision.ChangeImpact) {
			decision.ChangeImpact = classification.Impact
		}
	}
	sort.Slice(decision.Classifications, func(i, j int) bool {
		return decision.Classifications[i].ChangeID.String() < decision.Classifications[j].ChangeID.String()
	})

	current := parseSemver(request.CurrentVersion.String())
	candidate := parseSemver(request.CandidateVersion.String())
	decision.RequiredTransition = transitionForImpact(decision.ChangeImpact)
	if decision.ChangeImpact == curriculum.PackImpactMajor && current.major == "0" && candidate.major == "0" {
		decision.RequiredTransition = curriculum.PackTransitionMinor
	}
	decision.ActualTransition = actualPackTransition(current, candidate)

	switch {
	case compareSemver(current, candidate) >= 0:
		decision.ActualTransition = curriculum.PackTransitionInvalid
		decision.Reasons = []string{"candidate version must have greater SemVer precedence; published versions and build-metadata-only identities cannot be replaced"}
	case decision.ActualTransition == curriculum.PackTransitionPrerelease:
		decision.Allowed = true
		decision.Reasons = []string{"candidate is a new, increasing prerelease on the already-classified core release line"}
	case decision.ActualTransition == curriculum.PackTransitionInvalid:
		decision.Reasons = []string{"candidate must increment exactly one required core component and reset lower components"}
	case decision.ActualTransition != decision.RequiredTransition:
		decision.Reasons = []string{fmt.Sprintf("%s change impact requires a %s transition, got %s", decision.ChangeImpact, decision.RequiredTransition, decision.ActualTransition)}
	default:
		decision.Allowed = true
		if decision.ChangeImpact == curriculum.PackImpactMajor && decision.RequiredTransition == curriculum.PackTransitionMinor {
			decision.Reasons = []string{"breaking 0.x change correctly opens the next minor line while retaining major impact classification"}
		} else {
			decision.Reasons = []string{fmt.Sprintf("%s change impact matches the candidate %s transition", decision.ChangeImpact, decision.ActualTransition)}
		}
	}
	if err := decision.Validate(); err != nil {
		return curriculum.PackVersioningDecision{}, Invalid(operation, err)
	}
	return decision, nil
}

func classifyPackChange(change curriculum.CurriculumChange) curriculum.PackChangeClassification {
	impact := curriculum.PackImpactPatch
	reason := "metadata, clarification, or source refresh without structural change"
	switch change.Kind {
	case curriculum.ChangeConceptRemoved, curriculum.ChangeConceptSplit, curriculum.ChangeConceptMerged:
		impact = curriculum.PackImpactMajor
		reason = "concept identity or tracking contract changes"
	case curriculum.ChangeConceptAdded, curriculum.ChangePrerequisiteChanged,
		curriculum.ChangeHierarchyChanged, curriculum.ChangeStatusChanged,
		curriculum.ChangeEnvironmentChanged:
		impact = curriculum.PackImpactMinor
		reason = "compatible curriculum structure or capability expands"
	}
	migrationImpact := curriculum.PackImpactPatch
	migrationReason := ""
	switch change.Migration {
	case curriculum.MigrationRequiresRecompile:
		migrationImpact = curriculum.PackImpactMinor
		migrationReason = "change requires compatible curriculum recompilation"
	case curriculum.MigrationRequiresStudentReview, curriculum.MigrationBreaking:
		migrationImpact = curriculum.PackImpactMajor
		migrationReason = "change requires learner review or is explicitly breaking"
	}
	if packImpactRank(migrationImpact) > packImpactRank(impact) {
		impact = migrationImpact
		reason = migrationReason
	}
	return curriculum.PackChangeClassification{ChangeID: change.ID, Kind: change.Kind, Impact: impact, Reason: reason}
}

func packImpactRank(impact curriculum.PackChangeImpact) int {
	switch impact {
	case curriculum.PackImpactMajor:
		return 3
	case curriculum.PackImpactMinor:
		return 2
	default:
		return 1
	}
}

func transitionForImpact(impact curriculum.PackChangeImpact) curriculum.PackVersionTransition {
	switch impact {
	case curriculum.PackImpactMajor:
		return curriculum.PackTransitionMajor
	case curriculum.PackImpactMinor:
		return curriculum.PackTransitionMinor
	default:
		return curriculum.PackTransitionPatch
	}
}

type semverParts struct {
	major, minor, patch string
	prerelease          []string
}

func parseSemver(value string) semverParts {
	coreAndPre := strings.SplitN(strings.SplitN(value, "+", 2)[0], "-", 2)
	core := strings.Split(coreAndPre[0], ".")
	result := semverParts{major: core[0], minor: core[1], patch: core[2]}
	if len(coreAndPre) == 2 {
		result.prerelease = strings.Split(coreAndPre[1], ".")
	}
	return result
}

func actualPackTransition(current, candidate semverParts) curriculum.PackVersionTransition {
	if current.major == candidate.major && current.minor == candidate.minor && current.patch == candidate.patch {
		if len(current.prerelease) > 0 {
			return curriculum.PackTransitionPrerelease
		}
		return curriculum.PackTransitionInvalid
	}
	if current.major != candidate.major {
		if nextNumeric(current.major, candidate.major) && candidate.minor == "0" && candidate.patch == "0" {
			return curriculum.PackTransitionMajor
		}
		return curriculum.PackTransitionInvalid
	}
	if current.minor != candidate.minor {
		if nextNumeric(current.minor, candidate.minor) && candidate.patch == "0" {
			return curriculum.PackTransitionMinor
		}
		return curriculum.PackTransitionInvalid
	}
	if nextNumeric(current.patch, candidate.patch) {
		return curriculum.PackTransitionPatch
	}
	return curriculum.PackTransitionInvalid
}

func nextNumeric(current, candidate string) bool {
	currentNumber, candidateNumber := new(big.Int), new(big.Int)
	currentNumber.SetString(current, 10)
	candidateNumber.SetString(candidate, 10)
	return candidateNumber.Cmp(currentNumber.Add(currentNumber, big.NewInt(1))) == 0
}

func compareSemver(left, right semverParts) int {
	for _, pair := range [][2]string{{left.major, right.major}, {left.minor, right.minor}, {left.patch, right.patch}} {
		if compared := compareNumericIdentifier(pair[0], pair[1]); compared != 0 {
			return compared
		}
	}
	if len(left.prerelease) == 0 && len(right.prerelease) == 0 {
		return 0
	}
	if len(left.prerelease) == 0 {
		return 1
	}
	if len(right.prerelease) == 0 {
		return -1
	}
	for index := 0; index < len(left.prerelease) && index < len(right.prerelease); index++ {
		leftID, rightID := left.prerelease[index], right.prerelease[index]
		leftNumeric, rightNumeric := numericIdentifier(leftID), numericIdentifier(rightID)
		switch {
		case leftNumeric && rightNumeric:
			if compared := compareNumericIdentifier(leftID, rightID); compared != 0 {
				return compared
			}
		case leftNumeric:
			return -1
		case rightNumeric:
			return 1
		case leftID < rightID:
			return -1
		case leftID > rightID:
			return 1
		}
	}
	if len(left.prerelease) < len(right.prerelease) {
		return -1
	}
	if len(left.prerelease) > len(right.prerelease) {
		return 1
	}
	return 0
}

func compareNumericIdentifier(left, right string) int {
	leftNumber, rightNumber := new(big.Int), new(big.Int)
	leftNumber.SetString(left, 10)
	rightNumber.SetString(right, 10)
	return leftNumber.Cmp(rightNumber)
}

func numericIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

var _ PackVersioningService = PackVersioningPolicyV1{}
