//go:build windows

package wutil

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
)

var (
	onceStatus              sync.Once
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

func NewWSLCommand(arg ...string) *exec.Cmd {
	cmd := exec.Command("wsl", arg...)
	cmd.Env = append(os.Environ(), "WSL_UTF8=1")
	return cmd
}

func SilentExec(command string, args ...string) error {
	cmd := NewWSLCommand(args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command %s %v failed: %w", command, args, err)
	}
	return nil
}

func SilentExecCmd(args ...string) *exec.Cmd {
	cmd := NewWSLCommand(args...)
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
		cmd := SilentExecCmd("--status")
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

func IsWSLStoreVersionInstalled() bool {
	cmd := SilentExecCmd("--version")
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
	scanner := bufio.NewScanner(output)
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
