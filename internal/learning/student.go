package learning

import "fmt"

// ExperienceLevel is the student's self-reported or diagnosed starting point.
type ExperienceLevel string

const (
	ExperienceNovice       ExperienceLevel = "novice"
	ExperienceBeginner     ExperienceLevel = "beginner"
	ExperienceIntermediate ExperienceLevel = "intermediate"
	ExperienceAdvanced     ExperienceLevel = "advanced"
)

func (level ExperienceLevel) Valid() bool {
	switch level {
	case ExperienceNovice, ExperienceBeginner, ExperienceIntermediate, ExperienceAdvanced:
		return true
	default:
		return false
	}
}

// StudyPreference describes a preferred learning mode without tying it to a
// subject, content format, or user interface.
type StudyPreference string

const (
	PreferenceTheoryFirst StudyPreference = "theory_first"
	PreferencePractice    StudyPreference = "practice"
	PreferenceProjects    StudyPreference = "projects"
	PreferenceReflection  StudyPreference = "reflection"
)

func (preference StudyPreference) Valid() bool {
	switch preference {
	case PreferenceTheoryFirst, PreferencePractice, PreferenceProjects, PreferenceReflection:
		return true
	default:
		return false
	}
}

// Availability captures a sustainable weekly study budget. PreferredDays may
// contain values from time.Weekday encoded as 0 (Sunday) through 6 (Saturday).
type Availability struct {
	WeeklyMinutes int
	PreferredDays []int
}

func (availability Availability) Validate() error {
	if availability.WeeklyMinutes <= 0 {
		return fmt.Errorf("weekly availability must be positive")
	}
	seen := make(map[int]struct{}, len(availability.PreferredDays))
	for _, day := range availability.PreferredDays {
		if day < 0 || day > 6 {
			return fmt.Errorf("preferred day %d is invalid", day)
		}
		if _, exists := seen[day]; exists {
			return fmt.Errorf("preferred day %d is duplicated", day)
		}
		seen[day] = struct{}{}
	}
	return nil
}

// StudentProfile contains learner-provided characteristics used to personalize
// learning. It intentionally excludes credentials and unnecessary sensitive data.
type StudentProfile struct {
	DisplayName  string
	Experience   ExperienceLevel
	Preferences  []StudyPreference
	Availability Availability
}

func (profile StudentProfile) Validate() error {
	if err := requireText("student display name", profile.DisplayName); err != nil {
		return err
	}
	if !profile.Experience.Valid() {
		return fmt.Errorf("experience level %q is invalid", profile.Experience)
	}
	seen := make(map[StudyPreference]struct{}, len(profile.Preferences))
	for _, preference := range profile.Preferences {
		if !preference.Valid() {
			return fmt.Errorf("study preference %q is invalid", preference)
		}
		if _, exists := seen[preference]; exists {
			return fmt.Errorf("study preference %q is duplicated", preference)
		}
		seen[preference] = struct{}{}
	}
	return profile.Availability.Validate()
}

// Student is the root learner identity. Profile changes do not change its ID.
type Student struct {
	ID        ID
	Profile   StudentProfile
	CreatedAt Timestamp
	UpdatedAt Timestamp
}

func NewStudent(id ID, profile StudentProfile, createdAt Timestamp) (Student, error) {
	student := Student{ID: id, Profile: profile, CreatedAt: createdAt, UpdatedAt: createdAt}
	return student, student.Validate()
}

func (student Student) Validate() error {
	if err := student.ID.Validate(); err != nil {
		return fmt.Errorf("student: %w", err)
	}
	if err := student.Profile.Validate(); err != nil {
		return err
	}
	if err := student.CreatedAt.Validate(); err != nil {
		return fmt.Errorf("student created at: %w", err)
	}
	if err := student.UpdatedAt.Validate(); err != nil {
		return fmt.Errorf("student updated at: %w", err)
	}
	if student.UpdatedAt.Before(student.CreatedAt) {
		return fmt.Errorf("student update precedes creation")
	}
	return nil
}
