package memory

import "github.com/mishaaac/kelyro/internal/research/application"

var (
	_ application.SourceRepository           = sourceRepository{}
	_ application.SnapshotRepository         = snapshotRepository{}
	_ application.EvidenceRepository         = evidenceRepository{}
	_ application.ClaimRepository            = claimRepository{}
	_ application.ProvenanceRepository       = provenanceRepository{}
	_ application.ResearchRunRepository      = researchRunRepository{}
	_ application.TrustRegistryRepository    = trustRegistryRepository{}
	_ application.SourceRegistryRepository   = sourceRegistryRepository{}
	_ application.ReleaseRepository          = releaseRepository{}
	_ application.ReleaseIngestionRepository = releaseIngestionRepository{}
	_ application.FreshnessRepository        = freshnessRepository{}
	_ application.VerificationRepository     = verificationRepository{}
	_ application.ConflictRepository         = conflictRepository{}
	_ application.DriftRepository            = driftRepository{}
	_ application.ImpactRepository           = impactRepository{}
	_ application.ResearchCacheRepository    = cacheRepository{}
)
