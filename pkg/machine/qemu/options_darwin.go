package qemu

import (
	"os"

	"github.com/pkg/errors"
)

func getSocketDir() (string, error) {
	tmpDir, ok := os.LookupEnv("TMPDIR")
	if !ok {
		return "", errors.New("unable to resolve TMPDIR")
	}
	return tmpDir, nil
}
