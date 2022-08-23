//go:build !darwin
// +build !darwin

package qemu

func dockerClaimHelperInstalled() bool {
	return false
}

func claimDockerSock() bool {
	return false
}

func dockerClaimSupported() bool {
	return false
}

func findClaimHelper() string {
	return ""
}
