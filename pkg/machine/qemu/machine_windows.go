package qemu

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	gvproxy "github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/define"
)

func isProcessAlive(pid int) (bool, error) {
	if checkProcessStatus("process", pid, nil) == nil {
		return true, nil
	}
	return false, nil
}

func pingProcess(pid int) (int, error) {
	alive, _ := isProcessAlive(pid)
	if !alive {
		return -1, nil
	}
	return pid, nil
}

func killProcess(pid int, force bool) error {
	machine.SendQuit(uint32(pid))
	return nil
}

func checkProcessStatus(processHint string, pid int, stderrBuf *bytes.Buffer) error {
	active, exitCode := machine.GetProcessState(pid)
	if !active {
		if stderrBuf != nil {
			return fmt.Errorf("%s exited unexpectedly, exit code: %d stderr: %s", processHint, exitCode, stderrBuf.String())
		} else {
			return fmt.Errorf("%s exited unexpectedly, exit code: %d", processHint, exitCode)
		}
	}
	return nil
}

func forwardPipeArgs(cmd *gvproxy.GvproxyCommand, name string, destPath string, identityPath string, user string) error {
	machinePipe := toPipeName(name)
	if !machine.PipeNameAvailable(machinePipe) {
		return fmt.Errorf("could not start api proxy since expected pipe is not available: %s", machinePipe)
	}
	cmd.AddForwardSock(fmt.Sprintf("npipe:////./pipe/%s", machinePipe))
	cmd.AddForwardDest(destPath)
	cmd.AddForwardUser(user)
	cmd.AddForwardIdentity(identityPath)
	return nil
}

func pathsFromVolume(volume string) []string {
	paths := strings.SplitN(volume, ":", 3)
	driveLetterMatcher := regexp.MustCompile(`^(?:\\\\[.?]\\)?[a-zA-Z]$`)
	if len(paths) > 1 && driveLetterMatcher.MatchString(paths[0]) {
		paths = strings.SplitN(volume, ":", 4)
		paths = append([]string{paths[0] + ":" + paths[1]}, paths[2:]...)
	}
	return paths
}

func extractTargetPath(paths []string) string {
	if len(paths) > 1 {
		return paths[1]
	}
	target := strings.ReplaceAll(paths[0], "\\", "/")
	target = strings.ReplaceAll(target, ":", "/")
	if strings.HasPrefix(target, "//./") || strings.HasPrefix(target, "//?/") {
		target = target[4:]
	}
	dedup := regexp.MustCompile(`//+`)
	return dedup.ReplaceAllLiteralString("/"+target, "/")
}

func podmanPipe(name string) *define.VMFile {
	return &define.VMFile{Path: `\\.\pipe\` + toPipeName(name)}
}

func toPipeName(name string) string {
	if !strings.HasPrefix(name, "qemu-podman") {
		if !strings.HasPrefix(name, "podman") {
			name = "podman-" + name
		}
		name = "qemu-" + name
	}
	return name
}
