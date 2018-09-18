// +build selinux,linux

package buildah

import (
	"github.com/opencontainers/runtime-tools/generate"
)

func setupSelinux(g *generate.Generator, processLabel, mountLabel string) {
	g.SetProcessSelinuxLabel(processLabel)
	g.SetLinuxMountLabel(mountLabel)
}
