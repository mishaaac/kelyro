package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
	"github.com/mishaaac/kelyro/internal/research/diversity"
)

func TestSourceDiversityServiceUsesPersistedTrustAndRegistryOrganization(t *testing.T) {
	t.Parallel()
	repositories := memory.New().Repositories()
	claim := appendVerificationClaim(t, repositories, "diversity-service", "Runtime Org", "Operations Institute")
	service := application.NewSourceDiversityService(
		repositories.Claims, repositories.Sources, repositories.TrustRegistry, repositories.SourceRegistry,
	)
	request := application.AssessSourceDiversityRequest{
		ClaimID: claim.ID,
		Annotations: []application.SourceDiversityAnnotation{
			{SourceID: claim.SourceIDs[1], Perspective: diversity.PerspectiveIndependentReview, TechnicalRole: diversity.TechnicalRoleImplementation},
			{SourceID: claim.SourceIDs[0], Perspective: diversity.PerspectiveMaintainer, TechnicalRole: diversity.TechnicalRoleReference},
		},
	}
	got, err := service.Assess(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != diversity.StateSufficient || got.IndependentSourceCount != 2 ||
		got.OrganizationCount != 2 || !got.ReferencePresent || !got.ImplementationPresent {
		t.Fatalf("service assessment = %+v", got)
	}

	request.Annotations[0].DependencyGroup = "upstream:shared-guide"
	request.Annotations[1].DependencyGroup = "upstream:shared-guide"
	got, err = service.Assess(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != diversity.StateConcentrated || got.IndependentSourceCount != 1 || !containsDiversityWarning(got, diversity.WarningSharedDependency) {
		t.Fatalf("shared dependency assessment = %+v", got)
	}
}

func TestSourceDiversityServiceRequiresCompleteReviewedAnnotations(t *testing.T) {
	t.Parallel()
	repositories := memory.New().Repositories()
	claim := appendVerificationClaim(t, repositories, "diversity-incomplete", "Runtime Org", "Operations Institute")
	service := application.NewSourceDiversityService(
		repositories.Claims, repositories.Sources, repositories.TrustRegistry, repositories.SourceRegistry,
	)
	_, err := service.Assess(context.Background(), application.AssessSourceDiversityRequest{
		ClaimID: claim.ID,
		Annotations: []application.SourceDiversityAnnotation{{
			SourceID: claim.SourceIDs[0], Perspective: diversity.PerspectiveMaintainer,
			TechnicalRole: diversity.TechnicalRoleReference,
		}},
	})
	if !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("incomplete annotation error = %v, want invalid_state", err)
	}
}

func containsDiversityWarning(assessment diversity.Assessment, code diversity.WarningCode) bool {
	for _, warning := range assessment.Warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}
