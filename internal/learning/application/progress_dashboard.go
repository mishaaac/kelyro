package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
)

type ProgressDashboardOption func(*progressDashboardService)

func WithProgressDashboardClock(now func() time.Time) ProgressDashboardOption {
	return func(service *progressDashboardService) {
		if now != nil {
			service.now = now
		}
	}
}

func WithProgressDashboardAnalyticsPolicy(policy learning.LearningAnalyticsPolicy) ProgressDashboardOption {
	return func(service *progressDashboardService) { service.analyticsPolicy = policy }
}

type progressDashboardService struct {
	profiles        ProfileService
	mastery         MasteryPolicyService
	dailyPlan       AdaptiveDailyPlanService
	unitOfWork      UnitOfWork
	now             func() time.Time
	analyticsPolicy learning.LearningAnalyticsPolicy
}

func NewProgressDashboardService(profiles ProfileService, mastery MasteryPolicyService, dailyPlan AdaptiveDailyPlanService,
	unitOfWork UnitOfWork, options ...ProgressDashboardOption) ProgressDashboardService {
	service := &progressDashboardService{
		profiles: profiles, mastery: mastery, dailyPlan: dailyPlan, unitOfWork: unitOfWork, now: time.Now,
		analyticsPolicy: learning.DefaultLearningAnalyticsPolicy(),
	}
	for _, option := range options {
		option(service)
	}
	return service
}

type progressDashboardFacts struct {
	goal       *learning.LearningGoal
	instance   *learning.CurriculumInstance
	outline    []learning.CurriculumOutlineNode
	planning   []learning.DailyPlanCurriculumConcept
	states     []learning.InstanceConceptState
	retention  []learning.RetentionState
	reviews    []learning.ReviewItem
	sessions   []learning.StudySession
	history    []learning.StudyEvent
	milestones []learning.Milestone
}

func (service *progressDashboardService) Show(ctx context.Context) (ProgressDashboard, error) {
	const operation = "build progress dashboard"
	if service == nil || service.profiles == nil || service.mastery == nil || service.dailyPlan == nil || service.unitOfWork == nil || service.now == nil {
		return ProgressDashboard{}, Classify(ErrorUnavailable, operation, errors.New("progress dashboard dependencies are not configured"))
	}
	if err := service.analyticsPolicy.Validate(); err != nil {
		return ProgressDashboard{}, invalid(operation, err)
	}
	student, err := service.profiles.Show(ctx)
	if err != nil {
		return ProgressDashboard{}, repositoryError(operation, err)
	}
	masteryRequirement, err := service.mastery.Show(ctx, nil)
	if err != nil {
		return ProgressDashboard{}, repositoryError(operation, err)
	}

	var todayPlan *learning.DailyPlan
	plan, err := service.dailyPlan.Today(ctx)
	if err == nil {
		todayPlan = &plan
	} else if !errors.Is(err, ErrNotFound) {
		return ProgressDashboard{}, repositoryError(operation, err)
	}
	generatedAt, err := learning.NewTimestamp(service.now())
	if err != nil {
		return ProgressDashboard{}, invalid(operation, fmt.Errorf("read progress dashboard clock: %w", err))
	}

	facts := progressDashboardFacts{}
	var analytics learning.LearningAnalyticsSnapshot
	err = service.unitOfWork.WithinTransaction(ctx, func(repositories Repositories) error {
		if err := requireProgressDashboardRepositories(operation, repositories); err != nil {
			return err
		}
		goals, err := repositories.Goals.ListByStudent(ctx, student.ID)
		if err != nil {
			return err
		}
		facts.goal, err = progressDashboardActiveGoal(goals)
		if err != nil {
			return Classify(ErrorInvalidState, operation, err)
		}
		facts.sessions, err = repositories.StudySessions.ListByStudent(ctx, student.ID)
		if err != nil {
			return err
		}
		facts.history, err = repositories.History.ListByStudent(ctx, student.ID, nil, nil)
		if err != nil {
			return err
		}
		if facts.goal != nil {
			instances, listErr := repositories.CurriculumInstances.ListByStudent(ctx, student.ID)
			if listErr != nil {
				return listErr
			}
			facts.instance = progressDashboardActiveInstance(instances, facts.goal.ID)
			facts.milestones, err = repositories.Achievements.ListMilestones(ctx, student.ID, facts.goal.ID)
			if err != nil {
				return err
			}
		}
		if facts.instance != nil {
			facts.outline, err = repositories.Curricula.Outline(ctx, facts.instance.Curriculum)
			if err != nil {
				return err
			}
			facts.planning, err = repositories.Curricula.PlanningConcepts(ctx, facts.instance.Curriculum)
			if err != nil {
				return err
			}
			facts.states, err = repositories.InstanceConceptStates.ListByInstance(ctx, facts.instance.ID)
			if err != nil {
				return err
			}
			allReviews, listErr := repositories.Reviews.ListByStudent(ctx, student.ID)
			if listErr != nil {
				return listErr
			}
			allRetention, listErr := repositories.Retention.ListByStudent(ctx, student.ID)
			if listErr != nil {
				return listErr
			}
			knownConcepts, indexErr := progressDashboardConceptSet(facts.outline)
			if indexErr != nil {
				return Classify(ErrorInvalidState, operation, indexErr)
			}
			facts.reviews = progressDashboardReviews(allReviews, knownConcepts)
			facts.retention = progressDashboardRetention(allRetention, knownConcepts)
		}
		analytics, err = learning.CalculateLearningAnalyticsV1(learning.LearningAnalyticsInput{
			StudentID: student.ID, Timezone: student.Profile.Timezone, AsOf: generatedAt,
			ConceptStates: facts.states, RetentionStates: facts.retention, Reviews: facts.reviews,
			Sessions: facts.sessions, Events: facts.history, Policy: service.analyticsPolicy,
		})
		if err != nil {
			return Classify(ErrorInvalidState, operation, err)
		}
		return nil
	})
	if err != nil {
		return ProgressDashboard{}, repositoryError(operation, err)
	}
	if todayPlan != nil {
		if facts.goal == nil || facts.instance == nil || todayPlan.StudentID != student.ID ||
			todayPlan.GoalID != facts.goal.ID || todayPlan.CurriculumInstanceID != facts.instance.ID {
			return ProgressDashboard{}, Classify(ErrorInvalidState, operation, errors.New("today plan does not match the active learning context"))
		}
	}
	return assembleProgressDashboard(student, generatedAt, facts, analytics, masteryRequirement, todayPlan)
}

func requireProgressDashboardRepositories(operation string, repositories Repositories) error {
	if repositories.Goals == nil || repositories.CurriculumInstances == nil || repositories.Curricula == nil ||
		repositories.InstanceConceptStates == nil || repositories.Retention == nil || repositories.Reviews == nil ||
		repositories.StudySessions == nil || repositories.History == nil || repositories.Achievements == nil {
		return Classify(ErrorUnavailable, operation, errors.New("progress dashboard repositories are not configured"))
	}
	return nil
}

func progressDashboardActiveGoal(goals []learning.LearningGoal) (*learning.LearningGoal, error) {
	var active *learning.LearningGoal
	for index := range goals {
		if goals[index].Status != learning.GoalActive {
			continue
		}
		if active != nil {
			return nil, errors.New("multiple active learning goals")
		}
		candidate := goals[index]
		active = &candidate
	}
	return active, nil
}

func progressDashboardActiveInstance(instances []learning.CurriculumInstance, goalID learning.ID) *learning.CurriculumInstance {
	candidates := make([]learning.CurriculumInstance, 0)
	for _, instance := range instances {
		if instance.GoalID == goalID && instance.Status == learning.CurriculumInstanceActive {
			candidates = append(candidates, instance)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].UpdatedAt != candidates[j].UpdatedAt {
			return candidates[i].UpdatedAt.After(candidates[j].UpdatedAt)
		}
		return candidates[i].ID.String() > candidates[j].ID.String()
	})
	return &candidates[0]
}

func progressDashboardConceptSet(outline []learning.CurriculumOutlineNode) (map[learning.ID]struct{}, error) {
	result := make(map[learning.ID]struct{})
	seenNodes := make(map[learning.ID]struct{}, len(outline))
	for _, node := range outline {
		if err := node.Validate(); err != nil {
			return nil, err
		}
		if _, exists := seenNodes[node.ID]; exists {
			return nil, fmt.Errorf("dashboard curriculum contains duplicate node %q", node.ID)
		}
		seenNodes[node.ID] = struct{}{}
		if node.Type == learning.CurriculumNodeConcept {
			result[node.ID] = struct{}{}
		}
	}
	return result, nil
}

func progressDashboardReviews(items []learning.ReviewItem, concepts map[learning.ID]struct{}) []learning.ReviewItem {
	result := make([]learning.ReviewItem, 0)
	for _, item := range items {
		if _, exists := concepts[item.ConceptID]; exists {
			result = append(result, item)
		}
	}
	return result
}

func progressDashboardRetention(items []learning.RetentionState, concepts map[learning.ID]struct{}) []learning.RetentionState {
	result := make([]learning.RetentionState, 0)
	for _, item := range items {
		if _, exists := concepts[item.ConceptID]; exists {
			result = append(result, item)
		}
	}
	return result
}

func assembleProgressDashboard(student learning.Student, generatedAt learning.Timestamp, facts progressDashboardFacts,
	analytics learning.LearningAnalyticsSnapshot, masteryRequirement learning.ResolvedMasteryThreshold,
	todayPlan *learning.DailyPlan) (ProgressDashboard, error) {
	view := ProgressDashboard{
		StudentID: student.ID, GeneratedAt: generatedAt, Timezone: student.Profile.Timezone,
		ReviewsDue: analytics.Progress.ReviewsDue, StudyTime: analytics.Time, Streak: analytics.Activity,
		TodayPlan: todayPlan, MasteryRequirement: masteryRequirement, AnalyticsVersion: analytics.PolicyVersion,
		ReadModelVersion: ProgressDashboardReadModelVersion, WeakConcepts: []DashboardWeakConcept{},
	}
	view.OverallProgress = DashboardOverallProgress{
		ConceptsTotal:      learning.AnalyticsCountMetric{Description: "Total concepts counts the concepts in the active curriculum version."},
		ConceptsIntroduced: analytics.Progress.ConceptsIntroduced,
		ConceptsLearning:   analytics.Progress.ConceptsLearning,
		ConceptsMastered:   analytics.Progress.ConceptsMastered,
		Completion:         learning.AnalyticsRateMetric{Unit: "%", Description: "Completion is currently mastered concepts divided by all concepts in the active curriculum."},
	}
	view.Mastery = DashboardMasterySummary{
		KnownConcepts: learning.AnalyticsCountMetric{Value: analytics.Progress.ConceptsIntroduced.Value, Description: "Known concepts are active-curriculum concepts with at least one recorded exposure."},
		AverageKnown:  analytics.Mastery.AverageKnown,
	}
	if facts.goal != nil {
		goal := *facts.goal
		view.Goal = &goal
	}
	if facts.instance == nil {
		view.RecentMilestone = latestProgressDashboardMilestone(facts.milestones)
		return view, nil
	}
	concepts, titles, paths, err := progressDashboardOutline(facts.outline)
	if err != nil {
		return ProgressDashboard{}, Classify(ErrorInvalidState, "build progress dashboard", err)
	}
	instance := *facts.instance
	view.Curriculum = &DashboardCurriculum{Instance: instance, ConceptsTotal: len(concepts)}
	view.OverallProgress.ConceptsTotal.Value = len(concepts)
	if len(concepts) > 0 {
		view.OverallProgress.Completion.Value = 100 * float64(view.OverallProgress.ConceptsMastered.Value) / float64(len(concepts))
	}
	view.Current, err = progressDashboardCurrent(instance, concepts, paths, facts.states)
	if err != nil {
		return ProgressDashboard{}, Classify(ErrorInvalidState, "build progress dashboard", err)
	}
	view.Roadmap, err = progressDashboardRoadmap(facts.outline, facts.planning, facts.states, view.Current)
	if err != nil {
		return ProgressDashboard{}, Classify(ErrorInvalidState, "build progress dashboard", err)
	}
	for _, weak := range analytics.Mastery.Weakest.Concepts {
		title, exists := titles[weak.ConceptID]
		if !exists {
			return ProgressDashboard{}, Classify(ErrorInvalidState, "build progress dashboard", fmt.Errorf("weak concept %q is absent from curriculum outline", weak.ConceptID))
		}
		view.WeakConcepts = append(view.WeakConcepts, DashboardWeakConcept{
			CurriculumInstanceID: weak.CurriculumInstanceID, ConceptID: weak.ConceptID, Title: title, Mastery: weak.Mastery,
		})
	}
	view.RecentMilestone = latestProgressDashboardMilestone(facts.milestones)
	return view, nil
}

func progressDashboardRoadmap(outline []learning.CurriculumOutlineNode, planning []learning.DailyPlanCurriculumConcept,
	states []learning.InstanceConceptState, current *DashboardLocation) ([]DashboardRoadmapNode, error) {
	orderedOutline, err := progressDashboardOrderedOutline(outline)
	if err != nil {
		return nil, err
	}
	stateByConcept := make(map[learning.ID]learning.InstanceConceptState, len(states))
	for _, state := range states {
		if _, exists := stateByConcept[state.ConceptID]; exists {
			return nil, fmt.Errorf("duplicate curriculum concept state %q", state.ConceptID)
		}
		stateByConcept[state.ConceptID] = state
	}
	planningByConcept := make(map[learning.ID]learning.DailyPlanCurriculumConcept, len(planning))
	for _, concept := range planning {
		if err := concept.Validate(); err != nil {
			return nil, err
		}
		if _, exists := planningByConcept[concept.ConceptID]; exists {
			return nil, fmt.Errorf("duplicate dashboard planning concept %q", concept.ConceptID)
		}
		planningByConcept[concept.ConceptID] = concept
	}
	titles := make(map[learning.ID]string, len(outline))
	for _, node := range outline {
		titles[node.ID] = node.Title
	}
	currentID := learning.ID{}
	if current != nil {
		currentID = current.Concept.ID
	}
	result := make([]DashboardRoadmapNode, 0, len(orderedOutline))
	for _, node := range orderedOutline {
		item := DashboardRoadmapNode{
			ID: node.ID, Type: node.Type, Title: node.Title,
			ParentID: cloneDashboardID(node.ParentID), Depth: dashboardRoadmapDepth(node.Type),
			LockReasons: []string{},
		}
		if node.Type == learning.CurriculumNodeConcept {
			plan, exists := planningByConcept[node.ID]
			if !exists {
				return nil, fmt.Errorf("concept %q is absent from curriculum planning projection", node.ID)
			}
			state, statePresent := stateByConcept[node.ID]
			if statePresent && state.Exposure != learning.ExposureNotSeen {
				mastery := state.Mastery
				item.Mastery = &mastery
			}
			switch {
			case statePresent && state.Exposure == learning.ExposureReviewDue:
				item.Status = DashboardRoadmapReviewDue
			case statePresent && state.Exposure == learning.ExposureMastered:
				item.Status = DashboardRoadmapMastered
			default:
				item.LockReasons = dashboardRoadmapLockReasons(plan.PrerequisiteIDs, stateByConcept, titles)
				if len(item.LockReasons) > 0 {
					item.Status = DashboardRoadmapLocked
				} else if node.ID == currentID {
					item.Status = DashboardRoadmapCurrent
				} else {
					item.Status = DashboardRoadmapAvailable
				}
			}
		}
		result = append(result, item)
	}
	return result, nil
}

func progressDashboardOrderedOutline(outline []learning.CurriculumOutlineNode) ([]learning.CurriculumOutlineNode, error) {
	children := make(map[learning.ID][]learning.CurriculumOutlineNode)
	known := make(map[learning.ID]struct{}, len(outline))
	for _, node := range outline {
		if err := node.Validate(); err != nil {
			return nil, err
		}
		if _, exists := known[node.ID]; exists {
			return nil, fmt.Errorf("dashboard curriculum contains duplicate node %q", node.ID)
		}
		known[node.ID] = struct{}{}
		parentID := learning.ID{}
		if node.ParentID != nil {
			parentID = *node.ParentID
		}
		children[parentID] = append(children[parentID], node)
	}
	for _, node := range outline {
		if node.ParentID != nil {
			if _, exists := known[*node.ParentID]; !exists {
				return nil, fmt.Errorf("curriculum node %q has missing parent %q", node.ID, *node.ParentID)
			}
		}
	}
	for parentID := range children {
		sort.Slice(children[parentID], func(i, j int) bool {
			if children[parentID][i].Order != children[parentID][j].Order {
				return children[parentID][i].Order < children[parentID][j].Order
			}
			return children[parentID][i].ID.String() < children[parentID][j].ID.String()
		})
	}
	ordered := make([]learning.CurriculumOutlineNode, 0, len(outline))
	visiting := make(map[learning.ID]bool, len(outline))
	visited := make(map[learning.ID]bool, len(outline))
	var visit func(learning.CurriculumOutlineNode) error
	visit = func(node learning.CurriculumOutlineNode) error {
		if visiting[node.ID] {
			return fmt.Errorf("curriculum outline contains a cycle at %q", node.ID)
		}
		if visited[node.ID] {
			return nil
		}
		visiting[node.ID] = true
		ordered = append(ordered, node)
		for _, child := range children[node.ID] {
			if err := visit(child); err != nil {
				return err
			}
		}
		visiting[node.ID] = false
		visited[node.ID] = true
		return nil
	}
	for _, root := range children[learning.ID{}] {
		if err := visit(root); err != nil {
			return nil, err
		}
	}
	if len(ordered) != len(outline) {
		return nil, errors.New("curriculum outline is not connected to a root node")
	}
	return ordered, nil
}

func cloneDashboardID(source *learning.ID) *learning.ID {
	if source == nil {
		return nil
	}
	cloned := *source
	return &cloned
}

func dashboardRoadmapDepth(nodeType learning.CurriculumNodeType) int {
	switch nodeType {
	case learning.CurriculumNodePhase:
		return 0
	case learning.CurriculumNodeModule:
		return 1
	case learning.CurriculumNodeLesson:
		return 2
	case learning.CurriculumNodeTopic:
		return 3
	case learning.CurriculumNodeConcept:
		return 4
	default:
		return 0
	}
}

func dashboardRoadmapLockReasons(prerequisiteIDs []learning.ID, states map[learning.ID]learning.InstanceConceptState,
	titles map[learning.ID]string) []string {
	reasons := make([]string, 0)
	for _, prerequisiteID := range prerequisiteIDs {
		state, exists := states[prerequisiteID]
		if exists && (state.Exposure == learning.ExposureMastered || state.Exposure == learning.ExposureReviewDue) {
			continue
		}
		title := titles[prerequisiteID]
		if title == "" {
			title = prerequisiteID.String()
		}
		if !exists || state.Exposure == learning.ExposureNotSeen {
			reasons = append(reasons, fmt.Sprintf("Requires mastery of %s; current mastery is unknown.", title))
			continue
		}
		reasons = append(reasons, fmt.Sprintf("Requires mastery of %s; current mastery is %.0f%%.", title, state.Mastery.Value()*100))
	}
	return reasons
}

func progressDashboardOutline(outline []learning.CurriculumOutlineNode) ([]learning.ID, map[learning.ID]string, map[learning.ID][]learning.CurriculumOutlineNode, error) {
	byID := make(map[learning.ID]learning.CurriculumOutlineNode, len(outline))
	titles := make(map[learning.ID]string, len(outline))
	for _, node := range outline {
		if err := node.Validate(); err != nil {
			return nil, nil, nil, err
		}
		if _, exists := byID[node.ID]; exists {
			return nil, nil, nil, fmt.Errorf("dashboard curriculum contains duplicate node %q", node.ID)
		}
		byID[node.ID], titles[node.ID] = node, node.Title
	}
	paths := make(map[learning.ID][]learning.CurriculumOutlineNode)
	concepts := make([]learning.ID, 0)
	for _, node := range outline {
		if node.Type != learning.CurriculumNodeConcept {
			continue
		}
		path, err := progressDashboardPath(node, byID)
		if err != nil {
			return nil, nil, nil, err
		}
		paths[node.ID] = path
		concepts = append(concepts, node.ID)
	}
	sort.Slice(concepts, func(i, j int) bool { return progressDashboardPathBefore(paths[concepts[i]], paths[concepts[j]]) })
	return concepts, titles, paths, nil
}

func progressDashboardPath(node learning.CurriculumOutlineNode, byID map[learning.ID]learning.CurriculumOutlineNode) ([]learning.CurriculumOutlineNode, error) {
	path := []learning.CurriculumOutlineNode{node}
	seen := map[learning.ID]struct{}{node.ID: {}}
	for node.ParentID != nil {
		parent, exists := byID[*node.ParentID]
		if !exists {
			return nil, fmt.Errorf("curriculum node %q has missing parent %q", node.ID, *node.ParentID)
		}
		if _, exists := seen[parent.ID]; exists {
			return nil, fmt.Errorf("curriculum outline contains a cycle at %q", parent.ID)
		}
		seen[parent.ID] = struct{}{}
		path = append(path, parent)
		node = parent
	}
	for left, right := 0, len(path)-1; left < right; left, right = left+1, right-1 {
		path[left], path[right] = path[right], path[left]
	}
	want := []learning.CurriculumNodeType{
		learning.CurriculumNodePhase, learning.CurriculumNodeModule, learning.CurriculumNodeLesson,
		learning.CurriculumNodeTopic, learning.CurriculumNodeConcept,
	}
	if len(path) != len(want) {
		return nil, fmt.Errorf("concept %q has incomplete curriculum path", path[len(path)-1].ID)
	}
	for index := range want {
		if path[index].Type != want[index] {
			return nil, fmt.Errorf("concept %q has invalid %s path position %d", path[len(path)-1].ID, path[index].Type, index)
		}
	}
	return path, nil
}

func progressDashboardPathBefore(left, right []learning.CurriculumOutlineNode) bool {
	for index := 0; index < len(left) && index < len(right); index++ {
		if left[index].Order != right[index].Order {
			return left[index].Order < right[index].Order
		}
		if left[index].ID != right[index].ID {
			return left[index].ID.String() < right[index].ID.String()
		}
	}
	return len(left) < len(right)
}

func progressDashboardCurrent(instance learning.CurriculumInstance, concepts []learning.ID,
	paths map[learning.ID][]learning.CurriculumOutlineNode, states []learning.InstanceConceptState) (*DashboardLocation, error) {
	byConcept := make(map[learning.ID]learning.InstanceConceptState, len(states))
	for _, state := range states {
		if err := state.Validate(); err != nil {
			return nil, err
		}
		if state.CurriculumInstanceID != instance.ID || state.StudentID != instance.StudentID {
			return nil, fmt.Errorf("concept state %q belongs to another curriculum context", state.ConceptID)
		}
		if _, exists := byConcept[state.ConceptID]; exists {
			return nil, fmt.Errorf("duplicate curriculum concept state %q", state.ConceptID)
		}
		byConcept[state.ConceptID] = state
	}
	for _, conceptID := range concepts {
		state, exists := byConcept[conceptID]
		if exists && (state.Exposure == learning.ExposureMastered || state.Exposure == learning.ExposureReviewDue) {
			continue
		}
		path := paths[conceptID]
		return &DashboardLocation{
			Phase: dashboardNode(path[0]), Module: dashboardNode(path[1]), Lesson: dashboardNode(path[2]),
			Topic: dashboardNode(path[3]), Concept: dashboardNode(path[4]),
		}, nil
	}
	return nil, nil
}

func dashboardNode(node learning.CurriculumOutlineNode) DashboardNode {
	return DashboardNode{ID: node.ID, Title: node.Title}
}

func latestProgressDashboardMilestone(items []learning.Milestone) *learning.Milestone {
	if len(items) == 0 {
		return nil
	}
	latest := items[0]
	for _, item := range items[1:] {
		if item.ReachedAt.After(latest.ReachedAt) || (item.ReachedAt == latest.ReachedAt && item.ID.String() > latest.ID.String()) {
			latest = item
		}
	}
	return &latest
}

var _ ProgressDashboardService = (*progressDashboardService)(nil)
