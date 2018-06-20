package rootless

import (
	"fmt"
	"github.com/containers/storage/pkg/idtools"
	"os"
	"os/exec"
)

/*
extern int reexec_in_user_namespace(int ready);
extern int reexec_in_user_namespace_wait(int pid);
*/
import "C"

func runInUser() error {
	os.Setenv("_LIBPOD_USERNS_CONFIGURED", "done")
	return nil
}

func tryMappingTool(tool string, pid int, hostID int, mappings []idtools.IDMap) error {
	path, err := exec.LookPath(tool)
	if err != nil {
		return err
	}

	appendTriplet := func(l []string, a, b, c int) []string {
		return append(l, fmt.Sprintf("%d", a), fmt.Sprintf("%d", b), fmt.Sprintf("%d", c))
	}

	args := []string{path, fmt.Sprintf("%d", pid)}
	args = appendTriplet(args, 0, hostID, 1)
	if mappings != nil {
		for _, i := range mappings {
			args = appendTriplet(args, i.ContainerID+1, i.HostID, i.Size)
		}
	}
	cmd := exec.Cmd{
		Path: path,
		Args: args,
	}
	return cmd.Run()
}
