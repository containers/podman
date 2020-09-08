// +build !selinux !linux

package buildah

import (
	"github.com/opencontainers/runtime-tools/generate"
)

func selinuxGetEnabled() bool {
	return false
}

func setupSelinux(g *generate.Generator, processLabel, mountLabel string) {
}
