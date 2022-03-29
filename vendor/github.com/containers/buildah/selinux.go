//go:build linux
// +build linux

package buildah

import (
	"fmt"
	"os"

	"github.com/opencontainers/runtime-tools/generate"
	selinux "github.com/opencontainers/selinux/go-selinux"
	"github.com/pkg/errors"
)

func selinuxGetEnabled() bool {
	return selinux.GetEnabled()
}

func setupSelinux(g *generate.Generator, processLabel, mountLabel string) {
	if processLabel != "" && selinux.GetEnabled() {
		g.SetProcessSelinuxLabel(processLabel)
		g.SetLinuxMountLabel(mountLabel)
	}
}

func runLabelStdioPipes(stdioPipe [][]int, processLabel, mountLabel string) error {
	if !selinuxGetEnabled() || processLabel == "" || mountLabel == "" {
		// SELinux is completely disabled, or we're not doing anything at all with labeling
		return nil
	}
	pipeContext, err := selinux.ComputeCreateContext(processLabel, mountLabel, "fifo_file")
	if err != nil {
		return errors.Wrapf(err, "computing file creation context for pipes")
	}
	for i := range stdioPipe {
		pipeFdName := fmt.Sprintf("/proc/self/fd/%d", stdioPipe[i][0])
		if err := selinux.SetFileLabel(pipeFdName, pipeContext); err != nil && !os.IsNotExist(err) {
			return errors.Wrapf(err, "setting file label on %q", pipeFdName)
		}
	}
	return nil
}
