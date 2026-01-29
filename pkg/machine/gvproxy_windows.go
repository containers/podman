package machine

import (
	"errors"

	"go.podman.io/podman/v6/pkg/machine/define"
	"golang.org/x/sys/windows"
)

// removeGVProxyPIDFile special wrapper for deleting the GVProxyPIDFile on windows in case
// the file has an open handle which we will ignore.  unix does not have this problem
func removeGVProxyPIDFile(f define.VMFile) error {
	err := f.Delete()
	if err != nil && !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		return err
	}
	return nil
}
