//go:build !remote

package libpod

func checkCgroups2UnifiedMode(runtime *Runtime) {
}

func (r *Runtime) checkBootID(runtimeAliveFile string) error {
	return nil
}
