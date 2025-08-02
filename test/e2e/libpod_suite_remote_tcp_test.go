//go:build remote_testing && remote_tcp_testing && (linux || freebsd)

package integration

func PodmanTestCreate(tempDir string) *PodmanTestIntegration {
	pti := PodmanTestCreateUtil(tempDir, PodmanTestCreateUtilTargetTCP)
	pti.StartRemoteService()
	return pti
}
