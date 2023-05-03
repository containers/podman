package utils

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func FindTargetIdentityPath(sshDir string, srcIdentity string, prefix string, rangeStart int) (string, error) {
	identityFilepath := strings.TrimSpace(srcIdentity)
	if len(identityFilepath) == 0 {
		return FindAvailableIdentityPath(sshDir, prefix, rangeStart)
	} else if filepath.IsAbs(identityFilepath) {
		return identityFilepath, nil
	}
	return filepath.Join(sshDir, identityFilepath), nil
}

func FindAvailableIdentityPath(sshDir string, prefix string, rangeStart int) (string, error) {
	for i := 0; i < 65536; i++ {
		nextFilepath := filepath.Join(sshDir, fmt.Sprintf("%s-%04x", prefix, (rangeStart+i)%65536))
		_, err1 := os.Stat(nextFilepath)
		_, err2 := os.Stat(nextFilepath + ".pub")
		if errors.Is(err1, fs.ErrNotExist) && errors.Is(err2, fs.ErrNotExist) {
			return nextFilepath, nil
		}
	}
	return "", errors.New("can't determine available identity filename")
}
