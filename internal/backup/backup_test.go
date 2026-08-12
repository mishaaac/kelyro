package backup

import (
	"testing"
	"time"
)

func TestInfoFromManifestSummarizesFiles(t *testing.T) {
	t.Parallel()
	created := time.Date(2026, time.August, 12, 10, 0, 0, 0, time.UTC)
	info := InfoFromManifest(Manifest{
		ID: "backup-id", CreatedAt: created, Files: []File{{Size: 10}, {Size: 25}},
	})
	if info.ID != "backup-id" || info.CreatedAt != created || info.FileCount != 2 || info.TotalSize != 35 {
		t.Fatalf("InfoFromManifest() = %+v", info)
	}
}
