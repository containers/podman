//go:build !darwin

package shim

func findClaimHelper() string {
	return ""
}

func dockerClaimHelperInstalled() bool {
	return false
}

func claimDockerSock() bool {
	return false
}

func dockerClaimSupported() bool {
	return false
}
