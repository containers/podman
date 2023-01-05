package machine

import (
	"runtime"
	"syscall"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

func determineFedoraArch() string {
	const fallbackMsg = "this may result in the wrong Linux arch under emulation"
	var machine, native uint16
	current, _ := syscall.GetCurrentProcess()

	if err := windows.IsWow64Process2(windows.Handle(current), &machine, &native); err != nil {
		logrus.Warnf("Failure detecting native system architecture, %s: %w", fallbackMsg, err)
		// Fall-back to binary arch
		return runtime.GOARCH
	}

	switch native {
	// Only care about archs in use with WSL
	case 0xAA64:
		return "arm64"
	case 0x8664:
		return "amd64"
	default:
		logrus.Warnf("Unknown or unsupported native system architecture [%d], %s", fallbackMsg)
		return runtime.GOARCH
	}
}
