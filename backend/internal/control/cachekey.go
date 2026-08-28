package control

import (
	"crypto/sha256"
	"encoding/hex"
)

// CacheKey is an exact-match key over the outgoing request body (after any
// routing rewrite). Byte-identical bodies — and nothing else — share a key.
func CacheKey(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
