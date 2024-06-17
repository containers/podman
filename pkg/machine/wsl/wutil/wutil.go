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
	once    sync.Once
	wslPath string
)

func FindWSL() string {
	// At the time of this writing, a defect appeared in the OS preinstalled WSL executable
	// where it no longer reliably locates the preferred Windows App Store variant.
	//
	// Manually discover (and cache) the wsl.exe location to bypass the problem
	once.Do(func() {
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

func IsWSLInstalled() bool {
	cmd := SilentExecCmd(FindWSL(), "--status")
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
	cmd := SilentExecCmd(FindWSL(), "--version")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return false
	}

	return true
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
