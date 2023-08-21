//go:build windows
// +build windows

package wutil

import (
	"bufio"
	"io"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

func SilentExec(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func SilentExecCmd(command string, args ...string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	return cmd
}

func IsWSLInstalled() bool {
	cmd := SilentExecCmd("wsl", "--status")
	out, err := cmd.StdoutPipe()
	cmd.Stderr = nil
	if err != nil {
		return false
	}
	if err = cmd.Start(); err != nil {
		return false
	}

	kernelNotFound := matchOutputLine(out, "kernel file is not found")

	if err := cmd.Wait(); err != nil {
		return false
	}

	return !kernelNotFound
}

func IsWSLStoreVersionInstalled() bool {
	cmd := SilentExecCmd("wsl", "--version")
	out, err := cmd.StdoutPipe()
	cmd.Stderr = nil
	if err != nil {
		return false
	}
	if err = cmd.Start(); err != nil {
		return false
	}
	hasVersion := matchOutputLine(out, "WSL version:")
	if err := cmd.Wait(); err != nil {
		return false
	}

	return hasVersion
}

func matchOutputLine(output io.ReadCloser, match string) bool {
	scanner := bufio.NewScanner(transform.NewReader(output, unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, match) {
			return true
		}
	}
	return false
}
