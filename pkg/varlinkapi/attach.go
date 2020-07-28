// +build varlink

package varlinkapi

import (
	"bufio"
	"context"
	"io"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/libpod/events"
	iopodman "github.com/containers/podman/v2/pkg/varlink"
	"github.com/containers/podman/v2/pkg/varlinkapi/virtwriter"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/remotecommand"
)

func setupStreams(call iopodman.VarlinkCall) (*bufio.Reader, *bufio.Writer, *io.PipeReader, *io.PipeWriter, *define.AttachStreams) {

	// These are the varlink sockets
	reader := call.Call.Reader
	writer := call.Call.Writer

	// This pipe is used to pass stdin from the client to the input stream
	// once the msg has been "decoded"
	pr, pw := io.Pipe()

	stdoutWriter := virtwriter.NewVirtWriteCloser(writer, virtwriter.ToStdout)
	// TODO if runc ever starts passing stderr, we can too
	// stderrWriter := NewVirtWriteCloser(writer, ToStderr)

	streams := define.AttachStreams{
		OutputStream: stdoutWriter,
		InputStream:  bufio.NewReader(pr),
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
func (i *VarlinkAPI) Attach(call iopodman.VarlinkCall, name string, detachKeys string, start bool) error {
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
	if err := call.ReplyAttach(); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	reader, writer, _, pw, streams := setupStreams(call)
	go func() {
		if err := virtwriter.Reader(reader, nil, nil, pw, resize, nil); err != nil {
			errChan <- err
		}
	}()

	if state == define.ContainerStateRunning {
		finalErr = attach(ctr, streams, detachKeys, resize, errChan)
	} else {
		finalErr = startAndAttach(ctr, streams, detachKeys, resize, errChan)
	}

	exitCode := define.ExitCode(finalErr)
	if finalErr != define.ErrDetach && finalErr != nil {
		logrus.Error(finalErr)
	} else {
		if ecode, err := ctr.Wait(); err != nil {
			if errors.Cause(err) == define.ErrNoSuchCtr {
				// Check events
				event, err := i.Runtime.GetLastContainerEvent(context.Background(), ctr.ID(), events.Exited)
				if err != nil {
					logrus.Errorf("Cannot get exit code: %v", err)
					exitCode = define.ExecErrorCodeNotFound
				} else {
					exitCode = event.ContainerExitCode
				}
			} else {
				exitCode = define.ExitCode(err)
			}
		} else {
			exitCode = int(ecode)
		}
	}

	if ctr.AutoRemove() {
		err := i.Runtime.RemoveContainer(getContext(), ctr, false, false)
		if err != nil {
			logrus.Errorf("Failed to remove container %s: %s", ctr.ID(), err.Error())
		}
	}

	if err = virtwriter.HangUp(writer, uint32(exitCode)); err != nil {
		logrus.Errorf("Failed to HANG-UP attach to %s: %s", ctr.ID(), err.Error())
	}
	return call.Writer.Flush()
}

func attach(ctr *libpod.Container, streams *define.AttachStreams, detachKeys string, resize chan remotecommand.TerminalSize, errChan chan error) error {
	go func() {
		if err := ctr.Attach(streams, detachKeys, resize); err != nil {
			errChan <- err
		}
	}()
	attachError := <-errChan
	return attachError
}

func startAndAttach(ctr *libpod.Container, streams *define.AttachStreams, detachKeys string, resize chan remotecommand.TerminalSize, errChan chan error) error {
	var finalErr error
	attachChan, err := ctr.StartAndAttach(getContext(), streams, detachKeys, resize, false)
	if err != nil {
		return err
	}
	select {
	case attachChanErr := <-attachChan:
		finalErr = attachChanErr
	case chanError := <-errChan:
		finalErr = chanError
	}
	return finalErr
}
