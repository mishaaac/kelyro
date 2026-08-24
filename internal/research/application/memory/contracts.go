package memory

import "github.com/mishaaac/kelyro/internal/research/application"

var (
	_ application.SourceRepository         = sourceRepository{}
	_ application.SnapshotRepository       = snapshotRepository{}
	_ application.EvidenceRepository       = evidenceRepository{}
	_ application.ResearchRunRepository    = researchRunRepository{}
	_ application.TrustRegistryRepository  = trustRegistryRepository{}
	_ application.SourceRegistryRepository = sourceRegistryRepository{}
	_ application.ReleaseRepository        = releaseRepository{}
	_ application.FreshnessRepository      = freshnessRepository{}
	_ application.VerificationRepository   = verificationRepository{}
	_ application.DriftRepository          = driftRepository{}
	_ application.ImpactRepository         = impactRepository{}
	_ application.ResearchCacheRepository  = cacheRepository{}
)
