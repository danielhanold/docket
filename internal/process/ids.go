package process

import (
	"crypto/rand"
	"encoding/hex"
)

// NewRunIdentity allocates 128 bits of cryptographic randomness for the
// run ID and, independently, 128 for the token, each encoded as
// 32 lowercase hex characters. The token is the fallback reservation token:
// Launch writes it into the manifest only when the caller supplies no
// LaunchRequest.ReservationToken. Service.ResolveReservation reads the manifest
// token to resolve a lost launch response, but no clause of the ownership
// conjunction (filesystem capability, pid/pgid/sid identity, run-id/dirname
// agreement) consults it.
func NewRunIdentity() (runID, token string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", failf(FailExternal, "allocate-identity", "reading randomness: %v", err)
	}
	return hex.EncodeToString(buf[:16]), hex.EncodeToString(buf[16:]), nil
}
