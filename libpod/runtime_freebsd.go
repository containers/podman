//go:build !remote

package libpod


func checkCgroups2UnifiedMode(runtime *Runtime) {
	return
}

func warnIfNotTmpfs(paths []string) {
	return
}
