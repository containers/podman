package libpod

import (
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

type runcGlobalOptions struct {
	log           string
	logFormat     string
	root          string
	criu          string
	systemdCgroup bool
}
type runcExecOptions struct {
	consoleSocket string
	cwd           string
	env           []string
	tty           bool
	user          string
	processPath   string
	detach        bool
	pidFile       string
	processLabel  string
	apparmor      string
	noNewPrivs    bool
	capAdd        []string
}

func parseGlobalOptionsToArgs(opts runcGlobalOptions) []string {
	args := []string{}
	if opts.log != "" {
		args = append(args, "--log", opts.log)
	}
	if opts.logFormat != "" {
		args = append(args, "--log-format", opts.logFormat)
	}
	if opts.root != "" {
		args = append(args, "--root", opts.root)
	}
	if opts.criu != "" {
		args = append(args, "--criu", opts.criu)
	}
	if opts.systemdCgroup {
		args = append(args, "--systemd-cgroup")
	}
	return args
}

// RuncExec executes 'runc --options exec --options cmd'
func (r *OCIRuntime) RuncExec(container *Container, command []string, globalOpts runcGlobalOptions, execOpts runcExecOptions) error {
	args := []string{}
	args = append(args, parseGlobalOptionsToArgs(globalOpts)...)
	// Add subcommand
	args = append(args, "exec")
	// Now add subcommand args

	if execOpts.consoleSocket != "" {
		args = append(args, "--console-socket", execOpts.consoleSocket)
	}
	if execOpts.cwd != "" {
		args = append(args, "--cwd", execOpts.cwd)
	}

	if len(execOpts.env) > 0 {
		for _, envInput := range execOpts.env {
			args = append(args, "--env", envInput)
		}
	}
	if execOpts.tty {
		args = append(args, "--tty")
	}
	if execOpts.user != "" {
		args = append(args, "--user", execOpts.user)

	}
	if execOpts.processPath != "" {
		args = append(args, "--process", execOpts.processPath)
	}
	if execOpts.detach {
		args = append(args, "--detach")
	}
	if execOpts.pidFile != "" {
		args = append(args, "--pid-file", execOpts.pidFile)
	}
	if execOpts.processLabel != "" {
		args = append(args, "--process-label", execOpts.processLabel)
	}
	if execOpts.apparmor != "" {
		args = append(args, "--apparmor", execOpts.apparmor)
	}
	if execOpts.noNewPrivs {
		args = append(args, "--no-new-privs")
	}
	if len(execOpts.capAdd) > 0 {
		for _, capAddValue := range execOpts.capAdd {
			args = append(args, "--cap", capAddValue)
		}
	}

	// Append Cid
	args = append(args, container.ID())
	// Append Cmd
	args = append(args, command...)

	logrus.Debug("Executing runc command: %s %s", r.path, strings.Join(args, " "))
	cmd := exec.Command(r.path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Start()
	err := cmd.Wait()
	return err
}
