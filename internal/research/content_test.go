package research

import "testing"

func TestCanonicalContentHashV1(t *testing.T) {
	hash := CanonicalContentHashV1([]byte("first"))
	if hash != "sha256:a7937b64b8caa58f03721bb6bacf5c78cb235febe0e70b1b84cd99541461a08e" {
		t.Fatalf("CanonicalContentHashV1() = %q", hash)
	}
	if err := ValidateCanonicalContentHashV1(hash); err != nil {
		t.Fatalf("ValidateCanonicalContentHashV1() error = %v", err)
	}
	for _, invalid := range []string{"sha1:abc", "sha256:abc", "sha256:A7937B64B8CAA58F03721BB6BACF5C78CB235FEBE0E70B1B84CD99541461A08E"} {
		if err := ValidateCanonicalContentHashV1(invalid); err == nil {
			t.Fatalf("ValidateCanonicalContentHashV1(%q) accepted invalid hash", invalid)
		}
	}
}
