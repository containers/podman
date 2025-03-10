//go:build windows

package wutil

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/containers/storage/pkg/fileutils"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

var (
	onceFind, onceStatus    sync.Once
	wslPath                 string
	status                  wslStatus
	wslNotInstalledMessages = []string{"kernel file is not found", "The Windows Subsystem for Linux is not installed"}
	vmpDisabledMessages     = []string{"enable the Virtual Machine Platform Windows feature", "Enable \"Virtual Machine Platform\""}
	wslDisabledMessages     = []string{"enable the \"Windows Subsystem for Linux\" optional component"}
)

type wslStatus struct {
	installed         bool
	vmpFeatureEnabled bool
	wslFeatureEnabled bool
}

func FindWSL() string {
	// At the time of this writing, a defect appeared in the OS preinstalled WSL executable
	// where it no longer reliably locates the preferred Windows App Store variant.
	//
	// Manually discover (and cache) the wsl.exe location to bypass the problem
	onceFind.Do(func() {
		var locs []string

		// Prefer Windows App Store version
		if localapp := getLocalAppData(); localapp != "" {
			locs = append(locs, filepath.Join(localapp, "Microsoft", "WindowsApps", "wsl.exe"))
		}

		// Otherwise, the common location for the legacy system version
		root := os.Getenv("SystemRoot")
		if root == "" {
			root = `C:\Windows`
		}
		locs = append(locs, filepath.Join(root, "System32", "wsl.exe"))

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

func getLocalAppData() string {
	localapp := os.Getenv("LOCALAPPDATA")
	if localapp != "" {
		return localapp
	}

	if user := os.Getenv("USERPROFILE"); user != "" {
		return filepath.Join(user, "AppData", "Local")
	}

	return localapp
}

func SilentExec(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command %s %v failed: %w", command, args, err)
	}
	return nil
}

func SilentExecCmd(command string, args ...string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	return cmd
}

func parseWSLStatus() wslStatus {
	onceStatus.Do(func() {
		status = wslStatus{
			installed:         false,
			vmpFeatureEnabled: false,
			wslFeatureEnabled: false,
		}
		cmd := SilentExecCmd(FindWSL(), "--status")
		out, err := cmd.StdoutPipe()
		cmd.Stderr = nil
		if err != nil {
			return
		}
		if err = cmd.Start(); err != nil {
			return
		}

		status = matchOutputLine(out)

		if err := cmd.Wait(); err != nil {
			return
		}
	})

	return status
}

func IsWSLInstalled() bool {
	status := parseWSLStatus()
	return status.installed && status.vmpFeatureEnabled
}

func IsWSLFeatureEnabled() bool {
	if SilentExec(FindWSL(), "--set-default-version", "2") != nil {
		return false
	}
	status := parseWSLStatus()
	return status.vmpFeatureEnabled
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

func matchOutputLine(output io.ReadCloser) wslStatus {
	status := wslStatus{
		installed:         true,
		vmpFeatureEnabled: true,
		wslFeatureEnabled: true,
	}
	scanner := bufio.NewScanner(transform.NewReader(output, unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()))
	for scanner.Scan() {
		line := scanner.Text()
		for _, match := range wslNotInstalledMessages {
			if strings.Contains(line, match) {
				status.installed = false
			}
		}
		for _, match := range vmpDisabledMessages {
			if strings.Contains(line, match) {
				status.vmpFeatureEnabled = false
			}
		}
		for _, match := range wslDisabledMessages {
			if strings.Contains(line, match) {
				status.wslFeatureEnabled = false
			}
		}
	}
	return status
}
