package process

import (
	"crypto/rand"
	"encoding/hex"
)

// NewRunIdentity allocates 128 bits of cryptographic randomness for the
// run ID and, independently, 128 for the token, each encoded as
// 32 lowercase hex characters. The token is currently reserved and unused:
// it is written to the manifest but consumed by no clause of the ownership
// conjunction (which rests on filesystem capability, pid/pgid/sid identity,
// and run-id/dirname agreement). It is allocated for a possible future
// caller-presented-token check; no code path reads or verifies it today.
func NewRunIdentity() (runID, token string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", failf(FailExternal, "allocate-identity", "reading randomness: %v", err)
	}
	return hex.EncodeToString(buf[:16]), hex.EncodeToString(buf[16:]), nil
}
