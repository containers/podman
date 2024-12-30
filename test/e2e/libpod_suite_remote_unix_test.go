//go:build remote_testing && remote_unix_testing && (linux || freebsd)

package integration

func PodmanTestCreate(tempDir string) *PodmanTestIntegration {
	pti := PodmanTestCreateUtil(tempDir, PodmanTestCreateUtilTargetUnix)
	pti.StartRemoteService()
	return pti
}
