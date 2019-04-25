// +build varlink

package varlinkapi

import (
	"bufio"
	"io"

	"github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/varlinkapi/virtwriter"
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
func (i *LibpodAPI) Attach(call iopodman.VarlinkCall, name string, detachKeys string, start bool) error {
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
	if !start && state != libpod.ContainerStateRunning {
		return call.ReplyErrorOccurred("container must be running to attach")
	}
	reader, writer, _, pw, streams := setupStreams(call)

	go func() {
		if err := virtwriter.Reader(reader, nil, nil, pw, resize); err != nil {
			errChan <- err
		}
	}()

	if state == libpod.ContainerStateRunning {
		finalErr = attach(ctr, streams, detachKeys, resize, errChan)
	} else {
		finalErr = startAndAttach(ctr, streams, detachKeys, resize, errChan)
	}

	if finalErr != libpod.ErrDetach && finalErr != nil {
		logrus.Error(finalErr)
	}
	quitWriter := virtwriter.NewVirtWriteCloser(writer, virtwriter.Quit)
	_, err = quitWriter.Write([]byte("HANG-UP"))
	// TODO error handling is not quite right here yet
	return call.Writer.Flush()
}

func attach(ctr *libpod.Container, streams *libpod.AttachStreams, detachKeys string, resize chan remotecommand.TerminalSize, errChan chan error) error {
	go func() {
		if err := ctr.Attach(streams, detachKeys, resize); err != nil {
			errChan <- err
		}
	}()
	attachError := <-errChan
	return attachError
}

func startAndAttach(ctr *libpod.Container, streams *libpod.AttachStreams, detachKeys string, resize chan remotecommand.TerminalSize, errChan chan error) error {
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
