package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
)

const primaryStudentIDValue = "student.primary"

// ProfileOption configures deterministic collaborators for profile use cases.
type ProfileOption func(*profileService)

// WithProfileClock replaces the UTC clock used for profile timestamps.
func WithProfileClock(now func() time.Time) ProfileOption {
	return func(service *profileService) {
		if now != nil {
			service.now = now
		}
	}
}

type profileService struct {
	students StudentService
	now      func() time.Time
}

// NewProfileService creates the workspace-level profile use case over the
// existing StudentService boundary.
func NewProfileService(students StudentService, options ...ProfileOption) ProfileService {
	service := &profileService{students: students, now: time.Now}
	for _, option := range options {
		option(service)
	}
	return service
}

func (service *profileService) Show(ctx context.Context) (learning.Student, error) {
	const operation = "show student profile"
	if service == nil || service.students == nil {
		return learning.Student{}, Classify(ErrorUnavailable, operation, errors.New("student service is not configured"))
	}
	id, _ := learning.NewID(primaryStudentIDValue)
	student, err := service.students.Get(ctx, id)
	if err == nil {
		return student, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return learning.Student{}, err
	}
	timestamp, err := learning.NewTimestamp(service.now())
	if err != nil {
		return learning.Student{}, invalid(operation, err)
	}
	student, err = learning.NewStudent(id, learning.DefaultStudentProfile(), timestamp)
	if err != nil {
		return learning.Student{}, invalid(operation, err)
	}
	if err := service.students.Create(ctx, student); err != nil {
		if errors.Is(err, ErrConflict) {
			return service.students.Get(ctx, id)
		}
		return learning.Student{}, err
	}
	return student, nil
}

func (service *profileService) Edit(ctx context.Context, changes ProfileChanges) (learning.Student, error) {
	const operation = "edit student profile"
	if changes.empty() {
		return learning.Student{}, invalid(operation, errors.New("at least one profile field is required"))
	}
	student, err := service.Show(ctx)
	if err != nil {
		return learning.Student{}, err
	}
	profile := student.Profile
	applyProfileChanges(&profile, changes)
	timestamp, err := learning.NewTimestamp(service.now())
	if err != nil {
		return learning.Student{}, invalid(operation, err)
	}
	updated, err := student.UpdateProfile(profile, timestamp)
	if err != nil {
		return learning.Student{}, invalid(operation, err)
	}
	if err := service.students.Update(ctx, updated); err != nil {
		return learning.Student{}, err
	}
	return updated, nil
}

func (changes ProfileChanges) empty() bool {
	return changes.DisplayName == nil && changes.Experience == nil && changes.PreferredLanguage == nil &&
		changes.DailyMinutes == nil && changes.WeeklyDaysTarget == nil && changes.Preferences == nil && changes.Timezone == nil
}

func applyProfileChanges(profile *learning.StudentProfile, changes ProfileChanges) {
	if changes.DisplayName != nil {
		profile.DisplayName = strings.TrimSpace(*changes.DisplayName)
	}
	if changes.Experience != nil {
		profile.Experience = *changes.Experience
	}
	if changes.PreferredLanguage != nil {
		profile.PreferredLanguage = strings.TrimSpace(*changes.PreferredLanguage)
	}
	if changes.DailyMinutes != nil {
		profile.Availability.DailyMinutes = *changes.DailyMinutes
	}
	if changes.WeeklyDaysTarget != nil {
		profile.Availability.WeeklyDaysTarget = *changes.WeeklyDaysTarget
	}
	if changes.Preferences != nil {
		profile.Preferences = append([]learning.StudyPreference(nil), (*changes.Preferences)...)
	}
	if changes.Timezone != nil {
		profile.Timezone = strings.TrimSpace(*changes.Timezone)
	}
}
