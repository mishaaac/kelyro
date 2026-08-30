package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/mishaaac/kelyro/internal/research"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/queryplanner"
	triggerpolicy "github.com/mishaaac/kelyro/internal/research/trigger"
	trustpolicy "github.com/mishaaac/kelyro/internal/research/trust"
)

const researchCLIWorkflowV1 = "research-cli-workflow-v1"

// ResearchCLIView is a bounded, human-facing inspection model. A query plan
// contains discovery intentions only; it is never presented as evidence.
type ResearchCLIView struct {
	Request          research.ResearchRequest
	Run              research.ResearchRun
	Plan             *queryplanner.ResearchQueryPlan
	Bundle           *research.SourceBundle
	QueueItem        *research.ResearchQueueItem
	NetworkAllowed   bool
	DiscoveryPending bool
	AlgorithmVersion string
}

type SourceCLIView struct {
	Source         research.Source
	LatestSnapshot *research.SourceSnapshot
	TrustDecision  *research.TrustDecision
	Freshness      *researchapp.FreshnessRecord
}

type ResearchAuditCLIView struct {
	Request research.ResearchRequest
	Run     research.ResearchRun
	Records []research.ResearchRunAudit
}

func (service *Service) startResearchTopic(ctx context.Context, command Command, store researchapp.SourceRegistryStore) (ResearchCLIView, error) {
	subject := strings.Join(strings.Fields(command.ResearchTopic), " ")
	topic, err := research.NewResearchTopic(subject, "", "")
	if err != nil {
		return ResearchCLIView{}, fmt.Errorf("research topic: %w", err)
	}
	now, err := research.NewTimestamp(service.researchClock().UTC())
	if err != nil {
		return ResearchCLIView{}, fmt.Errorf("research clock: %w", err)
	}
	profileID, _ := research.NewID("authority.research-cli-default-v1")
	profile := research.AuthorityProfile{
		ID: profileID, Version: researchCLIWorkflowV1, Domain: "general", TopicPattern: "*",
		PreferredKinds:            []research.SourceKind{research.SourceOfficialDocumentation, research.SourceSpecification, research.SourcePaper},
		AllowedSupplementaryKinds: []research.SourceKind{research.SourceCommunityArticle},
		MinimumCorroboration:      2, MinimumTier: research.AuthorityTierC, CreatedAt: now,
	}
	plan, err := (queryplanner.PlannerV1{}).Plan(queryplanner.Input{
		Topic: topic, Purpose: research.PurposeCurrentUsage, AuthorityProfile: profile,
	})
	if err != nil {
		return ResearchCLIView{}, fmt.Errorf("build research query plan: %w", err)
	}
	settings, err := service.resolvedConfigForWorkspace(command.Workspace, command.ConfigOverrides)
	if err != nil {
		return ResearchCLIView{}, err
	}
	policy, err := policyFromSettings(settings)
	if err != nil {
		return ResearchCLIView{}, err
	}
	requestID, runID, queueID, auditID, err := newResearchCLIIDs()
	if err != nil {
		return ResearchCLIView{}, err
	}
	request := research.ResearchRequest{ID: requestID, Topic: topic, Purpose: research.PurposeCurrentUsage, RequestedAt: now}
	cost := research.ResearchCostMetadata{Budget: research.DefaultResearchCostBudgetV1(), AlgorithmVersion: research.ResearchCostControlAlgorithmV1}
	run := research.ResearchRun{ID: runID, RequestID: requestID, Status: research.ResearchRunPlanned, StartedAt: now, Cost: &cost}
	if store.Triggers() == nil {
		return ResearchCLIView{}, errors.New("research trigger service is unavailable")
	}
	decision, err := store.Triggers().Evaluate(ctx, triggerpolicy.Input{
		QueueID: queueID, Request: request,
		Signals: triggerpolicy.Signals{Manual: true, EvidenceCount: 0}, AsOf: now,
	})
	if err != nil {
		return ResearchCLIView{}, err
	}
	// Trigger deduplication may return the original queued request. Reuse that
	// immutable identity so repeated manual invocations become additional runs
	// of one logical request instead of diverging from their queue metadata.
	if decision.QueueItem != nil && decision.QueueItem.Request.ID != request.ID {
		request = decision.QueueItem.Request
		run.RequestID = request.ID
	}
	if store.Research() == nil {
		return ResearchCLIView{}, errors.New("research service is unavailable")
	}
	if err := store.Research().Start(ctx, request, run); err != nil {
		return ResearchCLIView{}, err
	}
	queries := make([]string, len(plan.Queries))
	for index, query := range plan.Queries {
		queries[index] = query.Query
	}
	audit, err := research.SealResearchRunAuditV1(research.ResearchRunAudit{
		ID: auditID, RunID: run.ID, RecordedAt: now, StartedAt: run.StartedAt, Outcome: run.Status,
		QueryPlannerVersion: plan.AlgorithmVersion, TrustPolicyVersion: trustpolicy.PolicyVersionV1,
		FreshnessVersion: research.FreshnessAlgorithmV1, ConflictResolverVersion: research.ConflictResolverAlgorithmV1,
		NetworkMode: research.ResearchAuditNetworkAuto, NetworkAllowed: policy.AllowNetwork, Queries: queries,
		TargetTechnology: request.Topic.Technology, TargetVersion: request.TargetVersion,
		AdditionalAlgorithms: []research.ResearchAuditAlgorithm{
			{Stage: "cost_control", Version: research.ResearchCostControlAlgorithmV1},
			{Stage: "research_trigger", Version: research.ResearchTriggerAlgorithmV1},
			{Stage: "workflow", Version: researchCLIWorkflowV1},
		},
	})
	if err != nil {
		return ResearchCLIView{}, fmt.Errorf("build initial research audit: %w", err)
	}
	if err := store.Research().RecordAudit(ctx, audit); err != nil {
		return ResearchCLIView{}, err
	}
	return ResearchCLIView{
		Request: request, Run: run, Plan: &plan, QueueItem: decision.QueueItem, NetworkAllowed: policy.AllowNetwork,
		DiscoveryPending: true, AlgorithmVersion: researchCLIWorkflowV1,
	}, nil
}

func newResearchCLIIDs() (research.ID, research.ID, research.ID, research.ID, error) {
	var entropy [12]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return research.ID{}, research.ID{}, research.ID{}, research.ID{}, fmt.Errorf("generate research identity: %w", err)
	}
	suffix := hex.EncodeToString(entropy[:])
	requestID, err := research.NewID("request.cli." + suffix)
	if err != nil {
		return research.ID{}, research.ID{}, research.ID{}, research.ID{}, err
	}
	runID, err := research.NewID("run.cli." + suffix)
	if err != nil {
		return research.ID{}, research.ID{}, research.ID{}, research.ID{}, err
	}
	queueID, err := research.NewID("queue.cli." + suffix)
	if err != nil {
		return research.ID{}, research.ID{}, research.ID{}, research.ID{}, err
	}
	auditID, err := research.NewID("audit.cli." + suffix + ".planned")
	return requestID, runID, queueID, auditID, err
}

func researchStatus(ctx context.Context, store researchapp.SourceRegistryStore, runID research.ID) (ResearchCLIView, error) {
	if store.Research() == nil {
		return ResearchCLIView{}, errors.New("research service is unavailable")
	}
	run, err := store.Research().Run(ctx, runID)
	if err != nil {
		return ResearchCLIView{}, err
	}
	request, err := store.Research().Request(ctx, run.RequestID)
	if err != nil {
		return ResearchCLIView{}, err
	}
	view := ResearchCLIView{Request: request, Run: run, AlgorithmVersion: researchCLIWorkflowV1}
	if store.Bundles() != nil {
		bundles, listErr := store.Bundles().ListForRun(ctx, runID)
		if listErr != nil {
			return ResearchCLIView{}, listErr
		}
		if len(bundles) > 0 {
			view.Bundle = &bundles[len(bundles)-1]
		}
	}
	return view, nil
}

func researchAudit(ctx context.Context, store researchapp.SourceRegistryStore, runID research.ID) (ResearchAuditCLIView, error) {
	if store.Research() == nil {
		return ResearchAuditCLIView{}, errors.New("research service is unavailable")
	}
	run, err := store.Research().Run(ctx, runID)
	if err != nil {
		return ResearchAuditCLIView{}, err
	}
	request, err := store.Research().Request(ctx, run.RequestID)
	if err != nil {
		return ResearchAuditCLIView{}, err
	}
	records, err := store.Research().AuditTrail(ctx, runID)
	if err != nil {
		return ResearchAuditCLIView{}, err
	}
	return ResearchAuditCLIView{Request: request, Run: run, Records: records}, nil
}
