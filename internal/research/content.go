package research

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const ContentHashAlgorithmV1 = "sha256"

// CanonicalContentHashV1 hashes the exact decoded response bytes delivered by
// the fetch adapter. Normalization has its own later content representation.
func CanonicalContentHashV1(content []byte) string {
	digest := sha256.Sum256(content)
	return ContentHashAlgorithmV1 + ":" + hex.EncodeToString(digest[:])
}

func ValidateCanonicalContentHashV1(value string) error {
	prefix := ContentHashAlgorithmV1 + ":"
	if !strings.HasPrefix(value, prefix) {
		return fmt.Errorf("content hash does not use %s", ContentHashAlgorithmV1)
	}
	encoded := strings.TrimPrefix(value, prefix)
	decoded, err := hex.DecodeString(encoded)
	if err != nil || len(decoded) != sha256.Size || encoded != strings.ToLower(encoded) {
		return fmt.Errorf("content hash is not canonical %s", ContentHashAlgorithmV1)
	}
	return nil
}
