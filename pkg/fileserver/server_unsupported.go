//go:build !windows

package fileserver

import (
	"fmt"
)

func StartShares(mounts map[string]string) error {
	if len(mounts) == 0 {
		return nil
	}

	return fmt.Errorf("this platform does not support sharing directories")
}
