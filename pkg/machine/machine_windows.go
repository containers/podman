//go:build windows
// +build windows

package machine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	pipePrefix     = "npipe:////./pipe/"
	globalPipe     = "docker_engine"
	winSShProxy    = "win-sshproxy.exe"
	winSshProxyTid = "win-sshproxy.tid"
	rootfulSock    = "/run/podman/podman.sock"
	rootlessSock   = "/run/user/1000/podman/podman.sock"
)

const WM_QUIT = 0x12 //nolint

type WinProxyOpts struct {
	Name           string
	IdentityPath   string
	Port           int
	RemoteUsername string
	Rootful        bool
	VMType         VMType
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

func PipeNameAvailable(pipeName string) bool {
	_, err := os.Stat(`\\.\pipe\` + pipeName)
	return os.IsNotExist(err)
}

func WaitPipeExists(pipeName string, retries int, checkFailure func() error) error {
	var err error
	for i := 0; i < retries; i++ {
		_, err = os.Stat(`\\.\pipe\` + pipeName)
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
	machinePipe := ToDist(opts.Name)
	if !PipeNameAvailable(machinePipe) {
		return false, "", fmt.Errorf("could not start api proxy since expected pipe is not available: %s", machinePipe)
	}

	globalName := false
	if PipeNameAvailable(globalPipe) {
		globalName = true
	}

	command, err := FindExecutablePeer(winSShProxy)
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
	args := []string{opts.Name, stateDir, pipePrefix + machinePipe, dest, opts.IdentityPath}
	waitPipe := machinePipe
	if globalName {
		args = append(args, pipePrefix+globalPipe, dest, opts.IdentityPath)
		waitPipe = globalPipe
	}

	cmd := exec.Command(command, args...)
	if err := cmd.Start(); err != nil {
		return globalName, "", err
	}

	return globalName, pipePrefix + waitPipe, WaitPipeExists(waitPipe, 80, func() error {
		active, exitCode := GetProcessState(cmd.Process.Pid)
		if !active {
			return fmt.Errorf("win-sshproxy.exe failed to start, exit code: %d (see windows event logs)", exitCode)
		}

		return nil
	})
}

func StopWinProxy(name string, vmtype VMType) error {
	pid, tid, tidFile, err := readWinProxyTid(name, vmtype)
	if err != nil {
		return err
	}

	proc, err := os.FindProcess(int(pid))
	if err != nil {
		return nil
	}
	sendQuit(tid)
	_ = waitTimeout(proc, 20*time.Second)
	_ = os.Remove(tidFile)

	return nil
}

func readWinProxyTid(name string, vmtype VMType) (uint32, uint32, string, error) {
	stateDir, err := GetWinProxyStateDir(name, vmtype)
	if err != nil {
		return 0, 0, "", err
	}

	tidFile := filepath.Join(stateDir, winSshProxyTid)
	contents, err := os.ReadFile(tidFile)
	if err != nil {
		return 0, 0, "", err
	}

	var pid, tid uint32
	fmt.Sscanf(string(contents), "%d:%d", &pid, &tid)
	return pid, tid, tidFile, nil
}

func waitTimeout(proc *os.Process, timeout time.Duration) bool {
	done := make(chan bool)
	go func() {
		proc.Wait()
		done <- true
	}()
	ret := false
	select {
	case <-time.After(timeout):
		proc.Kill()
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
	postMessage.Call(uintptr(tid), WM_QUIT, 0, 0)
}

func FindExecutablePeer(name string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}

	return filepath.Join(filepath.Dir(exe), name), nil
}

func GetWinProxyStateDir(name string, vmtype VMType) (string, error) {
	dir, err := GetDataDir(vmtype)
	if err != nil {
		return "", err
	}
	stateDir := filepath.Join(dir, name)
	if err = os.MkdirAll(stateDir, 0755); err != nil {
		return "", err
	}

	return stateDir, nil
}

func ToDist(name string) string {
	if !strings.HasPrefix(name, "podman") {
		name = "podman-" + name
	}
	return name
}
