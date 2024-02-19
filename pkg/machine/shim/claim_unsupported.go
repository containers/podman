//go:build !darwin

package shim

func findClaimHelper() string {
	return ""
}

// All of these are unused on Windows but are used on Linux.
// So we're just silencing Windows lint warnings here.

//nolint:unused
func dockerClaimHelperInstalled() bool {
	return false
}

//nolint:unused
func claimDockerSock() bool {
	return false
}

//nolint:unused
func dockerClaimSupported() bool {
	return false
}
