//go:build !darwin && !linux

package codexcontract

func ObserveRootIdentity(string, string) (RootIdentity, error) {
	return RootIdentity{}, ErrRootIdentityUnsupported
}
