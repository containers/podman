//go:build windows

package machine

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	winio "github.com/Microsoft/go-winio"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/env"
	"github.com/containers/podman/v5/pkg/machine/sockets"
	"github.com/containers/storage/pkg/fileutils"
	"github.com/sirupsen/logrus"
)

const (
	NamedPipePrefix = "npipe:////./pipe/"
	GlobalNamedPipe = "docker_engine"
	winSSHProxy     = "win-sshproxy.exe"
	winSSHProxyTid  = "win-sshproxy.tid"
	rootfulSock     = "/run/podman/podman.sock"
	rootlessSock    = "/run/user/1000/podman/podman.sock"

	// machine wait is longer since we must hard fail
	MachineNameWait = 5 * time.Second
	GlobalNameWait  = 250 * time.Millisecond
)

//nolint:stylecheck
const WM_QUIT = 0x12

type WinProxyOpts struct {
	Name           string
	IdentityPath   string
	Port           int
	RemoteUsername string
	Rootful        bool
	VMType         define.VMType
	Socket         *define.VMFile
}

func GetProcessState(pid int) (active bool, exitCode int) {
	const da = syscall.STANDARD_RIGHTS_READ | syscall.PROCESS_QUERY_INFORMATION | syscall.SYNCHRONIZE
	handle, err := syscall.OpenProcess(da, false, uint32(pid))
	if err != nil {
		logrus.Debugf("Error retrieving process %d: %v", pid, err)
		return false, int(syscall.ERROR_PROC_NOT_FOUND)
	}

	var code uint32
	if err := syscall.GetExitCodeProcess(handle, &code); err != nil {
		logrus.Errorf("Error retrieving process %d exit code: %v", pid, err)
	}
	return code == 259, int(code)
}

func PipeNameAvailable(pipeName string, maxWait time.Duration) bool {
	const interval = 250 * time.Millisecond
	var wait time.Duration
	for {
		err := fileutils.Exists(`\\.\pipe\` + pipeName)
		if errors.Is(err, fs.ErrNotExist) {
			return true
		}
		if wait >= maxWait {
			return false
		}
		time.Sleep(interval)
		wait += interval
	}
}

func WaitPipeExists(pipeName string, retries int, checkFailure func() error) error {
	var err error
	for i := 0; i < retries; i++ {
		err = fileutils.Exists(`\\.\pipe\` + pipeName)
		if err == nil {
			break
		}
		if fail := checkFailure(); fail != nil {
			return fail
		}
		time.Sleep(250 * time.Millisecond)
	}

	return err
}

func DialNamedPipe(ctx context.Context, path string) (net.Conn, error) {
	path = strings.ReplaceAll(path, "/", "\\")
	return winio.DialPipeContext(ctx, path)
}

func LaunchWinProxy(opts WinProxyOpts, noInfo bool) {
	globalName, pipeName, err := launchWinProxy(opts)
	if !noInfo {
		if err != nil {
			fmt.Fprintln(os.Stderr, "API forwarding for Docker API clients is not available due to the following startup failures.")
			fmt.Fprintf(os.Stderr, "\t%s\n", err.Error())
			fmt.Fprintln(os.Stderr, "\nPodman clients are still able to connect.")
		} else {
			fmt.Printf("API forwarding listening on: %s\n", pipeName)
			if globalName {
				fmt.Printf("\nDocker API clients default to this address. You do not need to set DOCKER_HOST.\n")
			} else {
				fmt.Printf("\nAnother process was listening on the default Docker API pipe address.\n")
				fmt.Printf("You can still connect Docker API clients by setting DOCKER HOST using the\n")
				fmt.Printf("following powershell command in your terminal session:\n")
				fmt.Printf("\n\t$Env:DOCKER_HOST = '%s'\n", pipeName)
				fmt.Printf("\nOr in a classic CMD prompt:\n")
				fmt.Printf("\n\tset DOCKER_HOST=%s\n", pipeName)
				fmt.Printf("\nAlternatively, terminate the other process and restart podman machine.\n")
			}
		}
	}
}

func launchWinProxy(opts WinProxyOpts) (bool, string, error) {
	machinePipe := env.WithPodmanPrefix(opts.Name)
	if !PipeNameAvailable(machinePipe, MachineNameWait) {
		return false, "", fmt.Errorf("could not start api proxy since expected pipe is not available: %s", machinePipe)
	}

	globalName := false
	if PipeNameAvailable(GlobalNamedPipe, GlobalNameWait) {
		globalName = true
	}

	command, err := FindExecutablePeer(winSSHProxy)
	if err != nil {
		return globalName, "", err
	}

	stateDir, err := GetWinProxyStateDir(opts.Name, opts.VMType)
	if err != nil {
		return globalName, "", err
	}

	destSock := rootlessSock
	forwardUser := opts.RemoteUsername

	if opts.Rootful {
		destSock = rootfulSock
		forwardUser = "root"
	}

	dest := fmt.Sprintf("ssh://%s@localhost:%d%s", forwardUser, opts.Port, destSock)
	args := []string{opts.Name, stateDir, NamedPipePrefix + machinePipe, dest, opts.IdentityPath}
	waitPipe := machinePipe
	if globalName {
		args = append(args, NamedPipePrefix+GlobalNamedPipe, dest, opts.IdentityPath)
		waitPipe = GlobalNamedPipe
	}

	hostURL, err := sockets.ToUnixURL(opts.Socket)
	if err != nil {
		return false, "", err
	}
	args = append(args, hostURL.String(), dest, opts.IdentityPath)

	cmd := exec.Command(command, args...)
	logrus.Debugf("winssh command: %s %v", command, args)
	if err := cmd.Start(); err != nil {
		return globalName, "", err
	}

	return globalName, NamedPipePrefix + waitPipe, WaitPipeExists(waitPipe, 80, func() error {
		active, exitCode := GetProcessState(cmd.Process.Pid)
		if !active {
			return fmt.Errorf("win-sshproxy.exe failed to start, exit code: %d (see windows event logs)", exitCode)
		}

		return nil
	})
}

func StopWinProxy(name string, vmtype define.VMType) error {
	pid, tid, tidFile, err := readWinProxyTid(name, vmtype)
	if err != nil {
		return err
	}

	proc, err := os.FindProcess(int(pid))
	if err != nil {
		//nolint:nilerr
		return nil
	}
	sendQuit(tid)
	_ = waitTimeout(proc, 20*time.Second)
	_ = os.Remove(tidFile)

	return nil
}

func readWinProxyTid(name string, vmtype define.VMType) (uint32, uint32, string, error) {
	stateDir, err := GetWinProxyStateDir(name, vmtype)
	if err != nil {
		return 0, 0, "", err
	}

	tidFile := filepath.Join(stateDir, winSSHProxyTid)
	contents, err := os.ReadFile(tidFile)
	if err != nil {
		return 0, 0, "", err
	}

	var pid, tid uint32
	_, err = fmt.Sscanf(string(contents), "%d:%d", &pid, &tid)
	return pid, tid, tidFile, err
}

func waitTimeout(proc *os.Process, timeout time.Duration) bool {
	done := make(chan bool)
	go func() {
		_, _ = proc.Wait()
		done <- true
	}()
	ret := false
	select {
	case <-time.After(timeout):
		_ = proc.Kill()
		<-done
	case <-done:
		ret = true
		break
	}

	return ret
}

func sendQuit(tid uint32) {
	user32 := syscall.NewLazyDLL("user32.dll")
	postMessage := user32.NewProc("PostThreadMessageW")
	//nolint:dogsled
	_, _, _ = postMessage.Call(uintptr(tid), WM_QUIT, 0, 0)
}

func FindExecutablePeer(name string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	exe, err = EvalSymlinksOrClean(exe)
	if err != nil {
		return "", err
	}

	return filepath.Join(filepath.Dir(exe), name), nil
}

func EvalSymlinksOrClean(filePath string) (string, error) {
	fileInfo, err := os.Lstat(filePath)
	if err != nil {
		return "", err
	}
	if fileInfo.Mode()&fs.ModeSymlink != 0 {
		// Only call filepath.EvalSymlinks if it is a symlink.
		// Starting with v1.23, EvalSymlinks returns an error for mount points.
		// See https://go-review.googlesource.com/c/go/+/565136 for reference.
		filePath, err = filepath.EvalSymlinks(filePath)
		if err != nil {
			return "", err
		}
	} else {
		// Call filepath.Clean when filePath is not a symlink. That's for
		// consistency with the symlink case (filepath.EvalSymlinks calls
		// Clean after evaluating filePath).
		filePath = filepath.Clean(filePath)
	}
	return filePath, nil
}

func GetWinProxyStateDir(name string, vmtype define.VMType) (string, error) {
	dir, err := env.GetDataDir(vmtype)
	if err != nil {
		return "", err
	}
	stateDir := filepath.Join(dir, name)
	if err = os.MkdirAll(stateDir, 0755); err != nil {
		return "", err
	}

	return stateDir, nil
}

func GetEnvSetString(env string, val string) string {
	return fmt.Sprintf("$Env:%s=\"%s\"", env, val)
}
