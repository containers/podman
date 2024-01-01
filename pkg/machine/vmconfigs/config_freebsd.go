package vmconfigs

import "os"

// Stubs
type HyperVConfig struct{}
type WSLConfig struct{}
type QEMUConfig struct{}
type AppleHVConfig struct{}

func getHostUID() int {
	return os.Getuid()
}
