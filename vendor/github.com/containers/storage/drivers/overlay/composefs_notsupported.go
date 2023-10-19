//go:build !linux || !composefs || !cgo
// +build !linux !composefs !cgo

package overlay

import (
	"fmt"
)

func composeFsSupported() bool {
	return false
}

func generateComposeFsBlob(verityDigests map[string]string, toc interface{}, composefsDir string) error {
	return fmt.Errorf("composefs is not supported")
}

func mountComposefsBlob(dataDir, mountPoint string) error {
	return fmt.Errorf("composefs is not supported")
}

func enableVerityRecursive(path string) (map[string]string, error) {
	return nil, fmt.Errorf("composefs is not supported")
}
