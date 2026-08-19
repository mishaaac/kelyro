package learning

import (
	"fmt"
	"strings"
	"time"
	_ "time/tzdata"
)

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

// Availability captures a sustainable daily budget and weekly target.
// PreferredDays may contain values from time.Weekday encoded as 0 (Sunday)
// through 6 (Saturday); they are optional even when WeeklyDaysTarget is set.
type Availability struct {
	DailyMinutes     int
	WeeklyDaysTarget int
	PreferredDays    []int
}

func (availability Availability) Validate() error {
	if availability.DailyMinutes < 5 || availability.DailyMinutes > 24*60 {
		return fmt.Errorf("daily time budget must be between 5 and 1440 minutes")
	}
	if availability.WeeklyDaysTarget < 1 || availability.WeeklyDaysTarget > 7 {
		return fmt.Errorf("weekly days target must be between 1 and 7")
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

// WeeklyMinutes returns the planning capacity implied by the profile. It is a
// convenience calculation, not an observed study metric.
func (availability Availability) WeeklyMinutes() int {
	return availability.DailyMinutes * availability.WeeklyDaysTarget
}

// StudentProfile contains learner-provided characteristics used to personalize
// learning. It intentionally excludes credentials and unnecessary sensitive data.
type StudentProfile struct {
	DisplayName       string
	Experience        ExperienceLevel
	PreferredLanguage string
	Preferences       []StudyPreference
	Availability      Availability
	Timezone          string
}

func (profile StudentProfile) Validate() error {
	if profile.DisplayName != "" && strings.TrimSpace(profile.DisplayName) == "" {
		return fmt.Errorf("student display name contains only whitespace")
	}
	if !profile.Experience.Valid() {
		return fmt.Errorf("experience level %q is invalid", profile.Experience)
	}
	if err := validateLanguage(profile.PreferredLanguage); err != nil {
		return err
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
	if err := profile.Availability.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(profile.Timezone) == "" || profile.Timezone != strings.TrimSpace(profile.Timezone) {
		return fmt.Errorf("student timezone is empty or padded")
	}
	if _, err := time.LoadLocation(profile.Timezone); err != nil {
		return fmt.Errorf("student timezone %q is invalid", profile.Timezone)
	}
	return nil
}

func validateLanguage(language string) error {
	if language == "" || language != strings.TrimSpace(language) {
		return fmt.Errorf("preferred language is empty or padded")
	}
	parts := strings.Split(language, "-")
	if len(parts[0]) < 2 || len(parts[0]) > 8 {
		return fmt.Errorf("preferred language %q is invalid", language)
	}
	for index, part := range parts {
		if len(part) < 1 || len(part) > 8 {
			return fmt.Errorf("preferred language %q is invalid", language)
		}
		for _, character := range part {
			letter := (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z')
			digit := character >= '0' && character <= '9'
			if !letter && (index == 0 || !digit) {
				return fmt.Errorf("preferred language %q is invalid", language)
			}
		}
	}
	return nil
}

// DefaultStudentProfile returns deterministic, privacy-preserving defaults.
// Locale and timezone are not inferred from the host so tests and new
// workspaces behave identically across machines.
func DefaultStudentProfile() StudentProfile {
	return StudentProfile{
		Experience:        ExperienceNovice,
		PreferredLanguage: "en",
		Availability: Availability{
			DailyMinutes:     30,
			WeeklyDaysTarget: 5,
		},
		Timezone: "UTC",
	}
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

// UpdateProfile replaces learner-provided profile data while preserving the
// student's stable identity and creation timestamp.
func (student Student) UpdateProfile(profile StudentProfile, updatedAt Timestamp) (Student, error) {
	student.Profile = profile
	student.UpdatedAt = updatedAt
	return student, student.Validate()
}
