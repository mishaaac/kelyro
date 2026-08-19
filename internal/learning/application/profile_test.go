package application_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/learning/application/memory"
)

func TestProfileServiceCreatesDefaultsAndAppliesPartialUpdates(t *testing.T) {
	t.Parallel()

	store := memory.New()
	now := time.Date(2026, time.August, 19, 10, 0, 0, 0, time.UTC)
	service := application.NewProfileService(
		application.NewStudentService(store.Repositories().Students),
		application.WithProfileClock(func() time.Time { return now }),
	)

	created, err := service.Show(context.Background())
	if err != nil {
		t.Fatalf("Show() error = %v", err)
	}
	if created.ID.String() != "student.primary" || !reflect.DeepEqual(created.Profile, learning.DefaultStudentProfile()) {
		t.Fatalf("Show() = %+v", created)
	}
	if reloaded, err := service.Show(context.Background()); err != nil || !reflect.DeepEqual(reloaded, created) {
		t.Fatalf("second Show() = (%+v, %v), want existing profile", reloaded, err)
	}

	now = now.Add(time.Hour)
	name, language, timezone := " Ada ", "es-PE", "America/Lima"
	experience := learning.ExperienceIntermediate
	dailyMinutes, weeklyDays := 45, 4
	preferences := []learning.StudyPreference{learning.PreferencePractice, learning.PreferenceReflection}
	updated, err := service.Edit(context.Background(), application.ProfileChanges{
		DisplayName: &name, Experience: &experience, PreferredLanguage: &language,
		DailyMinutes: &dailyMinutes, WeeklyDaysTarget: &weeklyDays,
		Preferences: &preferences, Timezone: &timezone,
	})
	if err != nil {
		t.Fatalf("Edit() error = %v", err)
	}
	if updated.Profile.DisplayName != "Ada" || updated.Profile.Experience != experience ||
		updated.Profile.PreferredLanguage != language || updated.Profile.Availability.DailyMinutes != dailyMinutes ||
		updated.Profile.Availability.WeeklyDaysTarget != weeklyDays || updated.Profile.Timezone != timezone {
		t.Fatalf("Edit() = %+v", updated.Profile)
	}
	if !updated.UpdatedAt.Time().Equal(now) || updated.CreatedAt == updated.UpdatedAt {
		t.Fatalf("Edit() timestamps = created %v updated %v", updated.CreatedAt.Time(), updated.UpdatedAt.Time())
	}

	emptyName := ""
	cleared, err := service.Edit(context.Background(), application.ProfileChanges{DisplayName: &emptyName})
	if err != nil || cleared.Profile.DisplayName != "" {
		t.Fatalf("clear display name = (%+v, %v)", cleared.Profile, err)
	}
}

func TestProfileServiceRejectsInvalidOrEmptyEdits(t *testing.T) {
	t.Parallel()

	store := memory.New()
	service := application.NewProfileService(application.NewStudentService(store.Repositories().Students))
	if _, err := service.Edit(context.Background(), application.ProfileChanges{}); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("Edit(empty) error = %v, want invalid state", err)
	}
	invalidTimezone := "Mars/Olympus"
	if _, err := service.Edit(context.Background(), application.ProfileChanges{Timezone: &invalidTimezone}); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("Edit(invalid timezone) error = %v, want invalid state", err)
	}
}
