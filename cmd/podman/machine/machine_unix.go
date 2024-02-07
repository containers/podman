//go:build linux || aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || netbsd || openbsd || solaris

package machine

import (
	"fmt"
	"os"

	"github.com/containers/podman/v5/pkg/rootless"
	"github.com/spf13/cobra"
)

func isUnixSocket(file os.DirEntry) bool {
	return file.Type()&os.ModeSocket != 0
}

func rootlessOnly(cmd *cobra.Command, args []string) error {
	if !rootless.IsRootless() {
		return fmt.Errorf("cannot run command %q as root", cmd.CommandPath())
	}
	return nil
}
