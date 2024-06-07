//go:build windows

package wutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/containers/storage/pkg/fileutils"
	"golang.org/x/sys/windows"
)

var (
	once    sync.Once
	wslPath string
)

const (
	// https://learn.microsoft.com/en-us/windows/win32/procthread/process-creation-flags
	flagsCreateNoWindow = 0x08000000
)

func FindWSL() string {
	// At the time of this writing, a defect appeared in the OS preinstalled WSL executable
	// where it no longer reliably locates the preferred Windows App Store variant.
	//
	// Manually discover (and cache) the wsl.exe location to bypass the problem
	once.Do(func() {
		var locs []string

		// Prefer Windows App Store version
		if p, ok := getLocalAppData(); ok {
			locs = append(locs, filepath.Join(p, "Microsoft", "WindowsApps", "wsl.exe"))
		}

		// Otherwise, the common location for the legacy system version
		locs = append(locs, filepath.Join(getSystem32Root(), "wsl.exe"))

		for _, loc := range locs {
			if err := fileutils.Exists(loc); err == nil {
				wslPath = loc
				return
			}
		}

		// Hope for the best
		wslPath = "wsl"
	})

	return wslPath
}

func SilentExec(command string, args ...string) error {
	cmd := SilentExecCmd(command, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command %s %v failed: %w", command, args, err)
	}
	return nil
}

func SilentExecCmd(command string, args ...string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: flagsCreateNoWindow}
	return cmd
}

func IsWSLInstalled() bool {
	// If the kernel file does not exist,
	// it means that the current system has only enabled the Features without running wsl --update
	if !existsKernel() {
		return false
	}

	if err := SilentExec(FindWSL(), "--status"); err != nil {
		return false
	}

	return true
}

func IsWSLStoreVersionInstalled() bool {
	cmd := SilentExecCmd(FindWSL(), "--version")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return false
	}

	return true
}

func existsKernel() bool {
	// from `MSI` or `Windows Update`
	kernel := filepath.Join(getSystem32Root(), "lxss", "tools", "kernel")
	if err := fileutils.Exists(kernel); err == nil {
		return true
	}

	// from `Microsoft Store` or `Github`
	kernel = filepath.Join(getProgramFiles(), "WSL", "tools", "kernel")
	if err := fileutils.Exists(kernel); err == nil {
		return true
	}

	return false
}

func getLocalAppData() (result string, ok bool) {
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		return local, true
	}

	if user := os.Getenv("USERPROFILE"); user != "" {
		return filepath.Join(user, "AppData", "Local"), true
	}

	return "", false
}

func getSystem32Root() string {
	if p := os.Getenv("SystemRoot"); p != "" {
		return filepath.Join(p, "System32")
	}

	if p, err := windows.GetSystemDirectory(); err == nil {
		return p
	}

	return `C:\Windows\System32`
}

func getProgramFiles() string {
	if p := os.Getenv("ProgramFiles"); p != "" {
		return p
	}

	return `C:\Program Files`
}
