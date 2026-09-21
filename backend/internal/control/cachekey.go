package control

import (
	"crypto/sha256"
	"encoding/hex"
)

func CacheKey(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
