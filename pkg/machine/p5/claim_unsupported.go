//build: !darwin

package p5

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
