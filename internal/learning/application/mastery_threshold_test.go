package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/learning/application/memory"
)

func TestMasteryPolicyServicePersistsLayersAndResolvesPrecedence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	clock := masteryPolicyClock()
	profiles := application.NewProfileService(application.NewStudentService(store.Repositories().Students), application.WithProfileClock(clock))
	service := application.NewMasteryPolicyService(profiles, store.Repositories().Mastery, application.WithMasteryPolicyClock(clock))

	resolved, err := service.Show(ctx, nil)
	if err != nil || resolved.Source != learning.MasterySourceStudentDefault || resolved.Requirement.Mode != learning.MasteryModeStandard {
		t.Fatalf("default Show() = (%+v, %v)", resolved, err)
	}
	relaxed, _ := learning.NewMasteryThreshold(.70)
	resolved, err = service.SetStudentDefault(ctx, relaxed)
	if err != nil || resolved.Requirement.Mode != learning.MasteryModeRelaxed || resolved.Source != learning.MasterySourceStudentDefault {
		t.Fatalf("SetStudentDefault() = (%+v, %v)", resolved, err)
	}
	strict, _ := learning.NewMasteryThreshold(.85)
	resolved, err = service.SetWorkspaceOverride(ctx, strict)
	if err != nil || resolved.Requirement.Mode != learning.MasteryModeStrict || resolved.Source != learning.MasterySourceWorkspaceOverride {
		t.Fatalf("SetWorkspaceOverride() = (%+v, %v)", resolved, err)
	}
	pack, _ := learning.NewPackMasteryOverride(.90, .80, .95)
	resolved, err = service.Show(ctx, &pack)
	if err != nil || resolved.Requirement.Mode != learning.MasteryModeMastery || resolved.Source != learning.MasterySourcePackOverride {
		t.Fatalf("pack Show() = (%+v, %v)", resolved, err)
	}
	resolved, err = service.ClearWorkspaceOverride(ctx)
	if err != nil || resolved.Requirement.Mode != learning.MasteryModeRelaxed || resolved.Source != learning.MasterySourceStudentDefault {
		t.Fatalf("ClearWorkspaceOverride() = (%+v, %v)", resolved, err)
	}

	reopened := application.NewMasteryPolicyService(profiles, store.Repositories().Mastery, application.WithMasteryPolicyClock(clock))
	resolved, err = reopened.Show(ctx, nil)
	if err != nil || resolved.Requirement.Mode != learning.MasteryModeRelaxed {
		t.Fatalf("persisted Show() = (%+v, %v)", resolved, err)
	}
}

func TestMasteryPolicyServiceRejectsProgressionValuesOutsideRange(t *testing.T) {
	t.Parallel()
	store := memory.New()
	clock := masteryPolicyClock()
	profiles := application.NewProfileService(application.NewStudentService(store.Repositories().Students), application.WithProfileClock(clock))
	service := application.NewMasteryPolicyService(profiles, store.Repositories().Mastery, application.WithMasteryPolicyClock(clock))
	tooLow, _ := learning.NewMasteryThreshold(.49)
	if _, err := service.SetWorkspaceOverride(context.Background(), tooLow); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("SetWorkspaceOverride(.49) error = %v", err)
	}
	resolved, err := service.Show(context.Background(), nil)
	if err != nil || resolved.Requirement.Mode != learning.MasteryModeStandard {
		t.Fatalf("invalid write changed settings: (%+v, %v)", resolved, err)
	}
}

func masteryPolicyClock() func() time.Time {
	current := time.Date(2026, time.August, 19, 10, 0, 0, 0, time.UTC)
	return func() time.Time {
		value := current
		current = current.Add(time.Minute)
		return value
	}
}
