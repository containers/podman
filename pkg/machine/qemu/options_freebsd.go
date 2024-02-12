//go:build freebsd

package qemu

import (
	"os"
)

func getRuntimeDir() (string, error) {
	tmpDir, ok := os.LookupEnv("TMPDIR")
	if !ok {
		tmpDir = "/tmp"
	}
	return tmpDir, nil
}

func useNetworkRecover() bool {
	return false
}
