package cni

import (
	"os/exec"
)

// FreeBSD vnet adds the lo0 interface automatically - we just need to
// add the default address. Note: this will also add ::1 as a side
// effect.
func setupLoopback(namespacePath string) error {
	// The jexec wrapper runs the ifconfig command inside the jail.
	return exec.Command("jexec", namespacePath, "ifconfig", "lo0", "inet", "127.0.0.1").Run()
}
