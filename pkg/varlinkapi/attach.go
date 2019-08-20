// +build varlink

package varlinkapi

import (
	"bufio"
	"io"

	"github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/libpod/events"
	"github.com/containers/libpod/pkg/varlinkapi/virtwriter"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/remotecommand"
)

func setupStreams(call iopodman.VarlinkCall) (*bufio.Reader, *bufio.Writer, *io.PipeReader, *io.PipeWriter, *libpod.AttachStreams) {

	// These are the varlink sockets
	reader := call.Call.Reader
	writer := call.Call.Writer

	// This pipe is used to pass stdin from the client to the input stream
	// once the msg has been "decoded"
	pr, pw := io.Pipe()

	stdoutWriter := virtwriter.NewVirtWriteCloser(writer, virtwriter.ToStdout)
	// TODO if runc ever starts passing stderr, we can too
	//stderrWriter := NewVirtWriteCloser(writer, ToStderr)

	streams := libpod.AttachStreams{
		OutputStream: stdoutWriter,
		InputStream:  pr,
		// Runc eats the error stream
		ErrorStream:  stdoutWriter,
		AttachInput:  true,
		AttachOutput: true,
		// Runc eats the error stream
		AttachError: true,
	}
	return reader, writer, pr, pw, &streams
}

// Attach connects to a containers console
func (i *LibpodAPI) Attach(call iopodman.VarlinkCall, name string, detachKeys string, start bool, exitCode int64) error {
	var finalErr error
	resize := make(chan remotecommand.TerminalSize)
	errChan := make(chan error)

	if !call.WantsUpgrade() {
		return call.ReplyErrorOccurred("client must use upgraded connection to attach")
	}
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	state, err := ctr.State()
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	if !start && state != define.ContainerStateRunning {
		return call.ReplyErrorOccurred("container must be running to attach")
	}

	// ACK the client upgrade request
	call.ReplyAttach()

	reader, writer, _, pw, streams := setupStreams(call)

	go func() {
		if err := virtwriter.Reader(reader, nil, nil, pw, resize, nil); err != nil {
			errChan <- err
		}
	}()
	if state == define.ContainerStateRunning {
		exitCode, finalErr = attach(ctr, streams, detachKeys, resize, errChan, i.Runtime, exitCode)
	} else {
		exitCode, finalErr = startAndAttach(ctr, streams, detachKeys, resize, errChan, i.Runtime, exitCode)
	}

	if err = virtwriter.HangUp(writer, uint32(exitCode)); err != nil {
		logrus.Errorf("Failed to HANG-UP attach to %s: %s", ctr.ID(), err.Error())
	}
	if err := call.Writer.Flush(); err != nil {
		logrus.Errorf("Attach Container writer flush err: %s", err.Error())
	}

	if finalErr != define.ErrDetach && finalErr != nil {
		logrus.Errorf("Attach %s failed with exitcode %d: %v", ctr.ID(), exitCode, finalErr)
		return finalErr
	}

	return nil
}

func attach(ctr *libpod.Container, streams *libpod.AttachStreams, detachKeys string, resize chan remotecommand.TerminalSize, errChan chan error, runtime *libpod.Runtime, exitCode int64) (int64, error) {
	go func() {
		if err := ctr.Attach(streams, detachKeys, resize); err != nil {
			errChan <- err
		}
	}()
	attachError := <-errChan
	if attachError != nil {
		logrus.Debugf("attach %s: %v", ctr.ID(), attachError)
		return exitCode, attachError
	}
	ecode, err := ctr.Wait()
	if err != nil {
		if errors.Cause(err) == define.ErrNoSuchCtr {
			// Check events
			event, err := runtime.GetLastContainerEvent(ctr.ID(), events.Exited)
			if err != nil {
				logrus.Errorf("Cannot get exit code: %v", err)
			} else {
				exitCode = int64(event.ContainerExitCode)
			}
		}
	} else {
		exitCode = int64(ecode)
	}
	logrus.Debugf("attach %s: %d:%v", ctr.ID(), exitCode, err)
	return exitCode, err
}

func startAndAttach(ctr *libpod.Container, streams *libpod.AttachStreams, detachKeys string, resize chan remotecommand.TerminalSize, errChan chan error, runtime *libpod.Runtime, exitCode int64) (int64, error) {
	var finalErr error
	attachChan, err := ctr.StartAndAttach(getContext(), streams, detachKeys, resize, false)
	if err != nil {
		logrus.Debugf("StartAndAttach %s failed: %v", ctr.ID(), err)
		return exitCode, err
	}
	select {
	case attachChanErr := <-attachChan:
		finalErr = attachChanErr
	case chanError := <-errChan:
		finalErr = chanError
	}
	if finalErr != nil {
		logrus.Debugf("StartAndAttach %s: %v", ctr.ID(), finalErr)
		return exitCode, finalErr
	}
	ecode, err := ctr.Wait()
	if err != nil {
		if errors.Cause(err) == define.ErrNoSuchCtr {
			// Check events
			event, err := runtime.GetLastContainerEvent(ctr.ID(), events.Exited)
			if err != nil {
				logrus.Errorf("Cannot get exit code: %v", err)
			} else {
				exitCode = int64(event.ContainerExitCode)
			}
		}
	} else {
		exitCode = int64(ecode)
	}
	logrus.Debugf("StartAndAttach %s: %d", ctr.ID(), exitCode)
	return exitCode, err
}
