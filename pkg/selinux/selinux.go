package util

import (
	"github.com/opencontainers/selinux/go-selinux"
)

// SELinuxKVMLabel returns labels for running kvm isolated containers
func SELinuxKVMLabel(cLabel string) (string, error) {
	if cLabel == "" {
		// selinux is disabled
		return "", nil
	}
	processLabel, _ := selinux.KVMContainerLabels()
	selinux.ReleaseLabel(processLabel)
	return swapSELinuxLabel(cLabel, processLabel)
}

// SELinuxInitLabel returns labels for running systemd based containers
func SELinuxInitLabel(cLabel string) (string, error) {
	if cLabel == "" {
		// selinux is disabled
		return "", nil
	}
	processLabel, _ := selinux.InitContainerLabels()
	selinux.ReleaseLabel(processLabel)
	return swapSELinuxLabel(cLabel, processLabel)
}

func swapSELinuxLabel(cLabel, processLabel string) (string, error) {
	dcon, err := selinux.NewContext(cLabel)
	if err != nil {
		return "", err
	}
	scon, err := selinux.NewContext(processLabel)
	if err != nil {
		return "", err
	}
	dcon["type"] = scon["type"]
	return dcon.Get(), nil
}
