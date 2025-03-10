//go:build remote_testing && remote_mtls_testing && (linux || freebsd)

package integration

func PodmanTestCreate(tempDir string) *PodmanTestIntegration {
	pti := PodmanTestCreateUtil(tempDir, PodmanTestCreateUtilTargetMTLS)
	pti.StartRemoteService()
	return pti
}
