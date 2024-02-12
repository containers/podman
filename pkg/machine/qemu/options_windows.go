//go:build windows

package qemu

import (
	"os"
)

func getRuntimeDir() (string, error) {
	tmpDir, ok := os.LookupEnv("TEMP")
	if !ok {
		tmpDir = os.Getenv("LOCALAPPDATA") + "\\Temp"
	}
	return tmpDir, nil
}

func useNetworkRecover() bool {
	return false
}
