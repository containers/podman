// +build varlink

package varlinkapi

import (
	"io"

	"github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/varlinkapi/virtwriter"
	"k8s.io/client-go/tools/remotecommand"
)

// Close is method to close the writer

// Attach ...
func (i *LibpodAPI) Attach(call iopodman.VarlinkCall, name string) error {
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

	go func() {
		if err := virtwriter.Reader(reader, nil, nil, pw, resize); err != nil {
			errChan <- err
		}
	}()

	go func() {
		// TODO allow for customizable detach keys
		if err := ctr.Attach(&streams, "", resize); err != nil {
			errChan <- err
		}
	}()

	select {
	// Blocking on an error
	case finalErr = <-errChan:
		// Need to close up shop
		_ = finalErr
	}
	quitWriter := virtwriter.NewVirtWriteCloser(writer, virtwriter.Quit)
	_, err = quitWriter.Write([]byte("HANG-UP"))
	return call.Writer.Flush()
}
