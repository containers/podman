/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package command

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// A generic command abstraction
type Command struct {
	cmds []*command
}

// The internal command representation
type command struct {
	*exec.Cmd
	pipeWriter *io.PipeWriter
}

// A generic command exit status
type Status struct {
	waitStatus syscall.WaitStatus
	*Stream
}

// Stream combines standard output and error
type Stream struct {
	stdOut string
	stdErr string
}

// New creates a new command from the provided arguments.
func New(cmd string, args ...string) *Command {
	return NewWithWorkDir("", cmd, args...)
}

// NewWithWorkDir creates a new command from the provided workDir and the command
// arguments.
func NewWithWorkDir(workDir, cmd string, args ...string) *Command {
	return &Command{
		cmds: []*command{{
			Cmd:        cmdWithDir(workDir, cmd, args...),
			pipeWriter: nil,
		}},
	}
}

func cmdWithDir(dir, cmd string, args ...string) *exec.Cmd {
	c := exec.Command(cmd, args...)
	c.Dir = dir
	return c
}

// Pipe creates a new command where the previous should be piped to
func (c *Command) Pipe(cmd string, args ...string) *Command {
	pipeCmd := cmdWithDir(c.cmds[0].Dir, cmd, args...)

	reader, writer := io.Pipe()
	c.cmds[len(c.cmds)-1].Stdout = writer
	pipeCmd.Stdin = reader

	c.cmds = append(c.cmds, &command{
		Cmd:        pipeCmd,
		pipeWriter: writer,
	})
	return c
}

// Run starts the command and waits for it to finish. It returns an error if
// the command execution was not possible at all, otherwise the Status.
// This method prints the commands output during execution
func (c *Command) Run() (res *Status, err error) {
	return c.run(true)
}

// RunSuccessOutput starts the command and waits for it to finish. It returns
// an error if the command execution was not successful, otherwise its output.
func (c *Command) RunSuccessOutput() (output *Stream, err error) {
	res, err := c.run(true)
	if err != nil {
		return nil, err
	}
	if !res.Success() {
		return nil, errors.Errorf("command %v did not succeed: %v", c.String(), res.Error())
	}
	return res.Stream, nil
}

// RunSuccess starts the command and waits for it to finish. It returns an
// error if the command execution was not successful.
func (c *Command) RunSuccess() error {
	_, err := c.RunSuccessOutput() // nolint: errcheck
	return err
}

// String returns a string representation of the full command
func (c *Command) String() string {
	str := []string{}
	for _, x := range c.cmds {
		// Note: the following logic can be replaced with x.String(), which was
		// implemented in go1.13
		b := new(strings.Builder)
		b.WriteString(x.Path)
		for _, a := range x.Args[1:] {
			b.WriteByte(' ')
			b.WriteString(a)
		}
		str = append(str, b.String())
	}
	return strings.Join(str, " | ")
}

// Run starts the command and waits for it to finish. It returns an error if
// the command execution was not possible at all, otherwise the Status.
// This method does not print the output of the command during its execution.
func (c *Command) RunSilent() (res *Status, err error) {
	return c.run(false)
}

// RunSilentSuccessOutput starts the command and waits for it to finish. It
// returns an error if the command execution was not successful, otherwise its
// output. This method does not print the output of the command during its
// execution.
func (c *Command) RunSilentSuccessOutput() (output *Stream, err error) {
	res, err := c.run(false)
	if err != nil {
		return nil, err
	}
	if !res.Success() {
		return nil, errors.Errorf("command %v did not succeed: %v", c.String(), res.Error())
	}
	return res.Stream, nil
}

// RunSilentSuccess starts the command and waits for it to finish. It returns
// an error if the command execution was not successful. This method does not
// print the output of the command during its execution.
func (c *Command) RunSilentSuccess() error {
	_, err := c.RunSilentSuccessOutput() // nolint: errcheck
	return err
}

// run is the internal run method
func (c *Command) run(printOutput bool) (res *Status, err error) {
	logrus.Debugf("Running command: %v", c.String())
	var runErr error
	stdOutBuffer := &bytes.Buffer{}
	stdErrBuffer := &bytes.Buffer{}
	status := &Status{Stream: &Stream{}}

	type done struct {
		stdout error
		stderr error
	}
	doneChan := make(chan done, 1)

	for i, cmd := range c.cmds {
		// Last command handling
		if i+1 == len(c.cmds) {
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				return nil, err
			}
			stderr, err := cmd.StderrPipe()
			if err != nil {
				return nil, err
			}

			var stdOutWriter, stdErrWriter io.Writer
			if printOutput {
				stdOutWriter = io.MultiWriter(os.Stdout, stdOutBuffer)
				stdErrWriter = io.MultiWriter(os.Stderr, stdErrBuffer)
			} else {
				stdOutWriter = stdOutBuffer
				stdErrWriter = stdErrBuffer
			}
			go func() {
				_, stdoutErr := io.Copy(stdOutWriter, stdout)
				_, stderrErr := io.Copy(stdErrWriter, stderr)
				doneChan <- done{stdoutErr, stderrErr}
			}()
		}

		if err := cmd.Start(); err != nil {
			return nil, err
		}

		if i > 0 {
			if err := c.cmds[i-1].Wait(); err != nil {
				return nil, err
			}
		}

		if cmd.pipeWriter != nil {
			if err := cmd.pipeWriter.Close(); err != nil {
				return nil, err
			}
		}

		// Wait for last command in the pipe to finish
		if i+1 == len(c.cmds) {
			err := <-doneChan
			if err.stdout != nil && strings.Contains(err.stdout.Error(), os.ErrClosed.Error()) {
				return nil, errors.Wrap(err.stdout, "unable to copy stdout")
			}
			if err.stderr != nil && strings.Contains(err.stderr.Error(), os.ErrClosed.Error()) {
				return nil, errors.Wrap(err.stderr, "unable to copy stderr")
			}

			runErr = cmd.Wait()
		}
	}

	status.stdOut = stdOutBuffer.String()
	status.stdErr = stdErrBuffer.String()

	if exitErr, ok := runErr.(*exec.ExitError); ok {
		if waitStatus, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			status.waitStatus = waitStatus
			return status, nil
		}
	}

	return status, runErr
}

// Success returns if a Status was successful
func (s *Status) Success() bool {
	return s.waitStatus.ExitStatus() == 0
}

// ExitCode returns the exit status of the command status
func (s *Status) ExitCode() int {
	return s.waitStatus.ExitStatus()
}

// Output returns stdout of the command status
func (s *Stream) Output() string {
	return s.stdOut
}

// Error returns the stderr of the command status
func (s *Stream) Error() string {
	return s.stdErr
}

// Execute is a convenience function which creates a new Command, executes it
// and evaluates its status.
func Execute(cmd string, args ...string) error {
	status, err := New(cmd, args...).Run()
	if err != nil {
		return errors.Wrapf(err, "command %q is not executable", cmd)
	}
	if !status.Success() {
		return errors.Errorf(
			"command %q did not exit successful (%d)",
			cmd, status.ExitCode(),
		)
	}
	return nil
}

// Available verifies that the specified `commands` are available within the
// current `$PATH` environment and returns true if so. The function does not
// check for duplicates nor if the provided slice is empty.
func Available(commands ...string) (ok bool) {
	ok = true
	for _, command := range commands {
		if _, err := exec.LookPath(command); err != nil {
			logrus.Warnf("Unable to %v", err)
			ok = false
		}
	}
	return ok
}
