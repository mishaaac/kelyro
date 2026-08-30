package research

import "testing"

func TestAllSourceRegistryStatusesAreValid(t *testing.T) {
	t.Parallel()
	for _, status := range []RegistryStatus{RegistryTrusted, RegistryConditional, RegistryHistorical, RegistryDeprecated, RegistryBlocked} {
		if err := status.Validate(); err != nil {
			t.Fatalf("RegistryStatus(%q).Validate() error = %v", status, err)
		}
	}
	if err := RegistryStatus("unknown").Validate(); err == nil {
		t.Fatal("RegistryStatus.Validate() accepted unknown status")
	}
}
