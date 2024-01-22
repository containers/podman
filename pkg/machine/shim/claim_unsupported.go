//build: !darwin

package shim

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
