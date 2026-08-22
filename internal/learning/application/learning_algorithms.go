package application

import (
	"fmt"
	"strings"

	"github.com/mishaaac/kelyro/internal/learning"
)

// MasteryAlgorithm is the replaceable calculation boundary used by bulk
// maintenance. Evidence remains immutable input regardless of implementation.
type MasteryAlgorithm interface {
	Version() string
	Calculate(learning.ID, learning.ID, []learning.Evidence) (learning.MasteryCalculation, error)
}

type RetentionAlgorithm interface {
	Version() string
	Calculate(learning.MasteryCalculation, []learning.Evidence, learning.Timestamp) (learning.RetentionCalculation, error)
}

type DailyPlanAlgorithm interface {
	Version() string
	Build(learning.DailyPlanInput) (learning.DailyPlan, error)
}

type LearningAlgorithmSuite struct {
	Mastery   MasteryAlgorithm
	Retention RetentionAlgorithm
	DailyPlan DailyPlanAlgorithm
}

func DefaultLearningAlgorithmSuite() LearningAlgorithmSuite {
	return LearningAlgorithmSuite{Mastery: masteryV1Algorithm{}, Retention: retentionV1Algorithm{}, DailyPlan: dailyPlanV1Algorithm{}}
}

func (suite LearningAlgorithmSuite) Validate() error {
	for _, candidate := range []struct {
		name      string
		algorithm interface{ Version() string }
	}{
		{name: "mastery", algorithm: suite.Mastery},
		{name: "retention", algorithm: suite.Retention},
		{name: "daily plan", algorithm: suite.DailyPlan},
	} {
		if candidate.algorithm == nil {
			return fmt.Errorf("%s algorithm is required", candidate.name)
		}
		if strings.TrimSpace(candidate.algorithm.Version()) == "" {
			return fmt.Errorf("%s algorithm version is empty", candidate.name)
		}
	}
	return nil
}

type masteryV1Algorithm struct{}

func (masteryV1Algorithm) Version() string { return learning.MasteryAlgorithmVersion }
func (masteryV1Algorithm) Calculate(studentID, conceptID learning.ID, evidence []learning.Evidence) (learning.MasteryCalculation, error) {
	return learning.CalculateMasteryV1(studentID, conceptID, evidence)
}

type retentionV1Algorithm struct{}

func (retentionV1Algorithm) Version() string { return learning.RetentionAlgorithmVersion }
func (retentionV1Algorithm) Calculate(mastery learning.MasteryCalculation, evidence []learning.Evidence, measuredAt learning.Timestamp) (learning.RetentionCalculation, error) {
	return learning.CalculateRetentionV1(mastery, evidence, measuredAt)
}

type dailyPlanV1Algorithm struct{}

func (dailyPlanV1Algorithm) Version() string { return learning.DailyPlanPolicyVersion }
func (dailyPlanV1Algorithm) Build(input learning.DailyPlanInput) (learning.DailyPlan, error) {
	return learning.BuildAdaptiveDailyPlanV1(input)
}
