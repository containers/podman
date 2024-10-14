//go:build linux && !cgo

package overlay

import (
	"fmt"
)

func openComposefsMount(dataDir string) (int, error) {
	return 0, fmt.Errorf("composefs not supported on this build")
}

func getComposeFsHelper() (string, error) {
	return "", fmt.Errorf("composefs not supported on this build")
}

func mountComposefsBlob(dataDir, mountPoint string) error {
	return fmt.Errorf("composefs not supported on this build")
}

func generateComposeFsBlob(verityDigests map[string]string, toc interface{}, composefsDir string) error {
	return fmt.Errorf("composefs not supported on this build")
}
