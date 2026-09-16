package application

import (
	"context"
	"errors"
	"testing"
)

func TestPackActivationCoordinatorV1HandsCurriculumToStudentCoreBeforeActivation(t *testing.T) {
	t.Parallel()
	pack := installationPack(t, "pack.backend", "1.0.0")
	repository := newInstallationRepositoryFake()
	repository.values[installationKey(pack)] = installedFixture(t, pack, "a")
	manager := NewPackInstallerV1(validationFake{}, NewPackDependencyResolverV1(), repository, fixedCurriculumClock(t, 30))
	students := &studentActivationFake{}
	service := NewPackActivationCoordinatorV1(manager, students)

	result, err := service.Activate(context.Background(), PackActivateRequest{WorkspaceRoot: "/workspace", PackID: pack.Manifest.ID, Version: pack.Manifest.Version})
	if err != nil {
		t.Fatal(err)
	}
	if students.preflight != 1 || students.applied != 1 || students.request.Curriculum.ID != pack.Curriculum.ID || result.PackID != pack.Manifest.ID {
		t.Fatalf("activation = %+v, student hand-off = %+v", result, students)
	}
	if _, err := manager.Active(context.Background(), "/workspace"); err != nil {
		t.Fatalf("active pack: %v", err)
	}
}

func TestPackActivationCoordinatorV1StopsBeforePackActivationWhenStudentPreflightFails(t *testing.T) {
	t.Parallel()
	pack := installationPack(t, "pack.backend", "1.0.0")
	repository := newInstallationRepositoryFake()
	repository.values[installationKey(pack)] = installedFixture(t, pack, "a")
	manager := NewPackInstallerV1(validationFake{}, NewPackDependencyResolverV1(), repository, fixedCurriculumClock(t, 30))
	students := &studentActivationFake{preflightErr: errors.New("no active learning goal")}
	service := NewPackActivationCoordinatorV1(manager, students)

	if _, err := service.Activate(context.Background(), PackActivateRequest{WorkspaceRoot: "/workspace", PackID: pack.Manifest.ID, Version: pack.Manifest.Version}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("Activate() error = %v", err)
	}
	if students.applied != 0 {
		t.Fatalf("student activation applied after failed preflight: %+v", students)
	}
	if _, err := manager.Active(context.Background(), "/workspace"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("pack activated after failed preflight: %v", err)
	}
}

func TestPackActivationCoordinatorV1RejectsDirectVersionTransitionBeforeStudentWrites(t *testing.T) {
	t.Parallel()
	current := installationPack(t, "pack.backend", "1.0.0")
	candidate := installationPack(t, "pack.backend", "1.1.0")
	repository := newInstallationRepositoryFake()
	repository.values[installationKey(current)] = installedFixture(t, current, "a")
	repository.values[installationKey(candidate)] = installedFixture(t, candidate, "b")
	manager := NewPackInstallerV1(validationFake{}, NewPackDependencyResolverV1(), repository, fixedCurriculumClock(t, 30))
	if _, err := manager.Activate(context.Background(), PackActivateRequest{WorkspaceRoot: "/workspace", PackID: current.Manifest.ID, Version: current.Manifest.Version}); err != nil {
		t.Fatal(err)
	}
	students := &studentActivationFake{}
	service := NewPackActivationCoordinatorV1(manager, students)

	if _, err := service.Activate(context.Background(), PackActivateRequest{WorkspaceRoot: "/workspace", PackID: candidate.Manifest.ID, Version: candidate.Manifest.Version}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("Activate() error = %v", err)
	}
	if students.preflight != 0 || students.applied != 0 {
		t.Fatalf("student hand-off ran before transition rejection: %+v", students)
	}
}

type studentActivationFake struct {
	request      StudentCurriculumActivationRequest
	preflight    int
	applied      int
	preflightErr error
}

func (fake *studentActivationFake) Preflight(_ context.Context, request StudentCurriculumActivationRequest) error {
	fake.preflight++
	fake.request = request
	return fake.preflightErr
}

func (fake *studentActivationFake) ActivateCurriculum(_ context.Context, request StudentCurriculumActivationRequest) (StudentCurriculumActivationResult, error) {
	fake.applied++
	fake.request = request
	return StudentCurriculumActivationResult{CurriculumInstanceID: "instance.backend", Created: true}, nil
}
