package process

import (
	"crypto/rand"
	"encoding/hex"
)

// NewRunIdentity allocates 128 bits of cryptographic randomness for the
// run ID and, independently, 128 for the ownership token, each encoded as
// 32 lowercase hex characters.
func NewRunIdentity() (runID, token string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", failf(FailExternal, "allocate-identity", "reading randomness: %v", err)
	}
	return hex.EncodeToString(buf[:16]), hex.EncodeToString(buf[16:]), nil
}
