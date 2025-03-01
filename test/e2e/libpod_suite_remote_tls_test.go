//go:build remote_testing && remote_tls_testing && (linux || freebsd)

package integration

func PodmanTestCreate(tempDir string) *PodmanTestIntegration {
	pti := PodmanTestCreateUtil(tempDir, PodmanTestCreateUtilTargetTLS)
	pti.StartRemoteService()
	return pti
}
