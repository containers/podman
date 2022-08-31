//go:build windows
// +build windows

package main

import (
	"C"
	"syscall"
	"unsafe"

	"github.com/containers/podman/v4/pkg/machine/wsl"
)

const KernelWarning = "WSL Kernel installation did not complete successfully. " +
	"Podman machine will attempt to install this at a later time. " +
	"You can also manually complete the installation using the " +
	"\"wsl --update\" command."

//export CheckWSL
func CheckWSL(hInstall uint32) uint32 {
	installed := wsl.IsWSLInstalled()
	feature := wsl.IsWSLFeatureEnabled()
	setMsiProperty(hInstall, "HAS_WSL", strBool(installed))
	setMsiProperty(hInstall, "HAS_WSLFEATURE", strBool(feature))

	return 0
}

func setMsiProperty(hInstall uint32, name string, value string) {
	nameW, _ := syscall.UTF16PtrFromString(name)
	valueW, _ := syscall.UTF16PtrFromString(value)

	msi := syscall.NewLazyDLL("msi")
	proc := msi.NewProc("MsiSetPropertyW")
	_, _, _ = proc.Call(uintptr(hInstall), uintptr(unsafe.Pointer(nameW)), uintptr(unsafe.Pointer(valueW)))

}
func strBool(val bool) string {
	if val {
		return "1"
	}

	return "0"
}

func main() {}
