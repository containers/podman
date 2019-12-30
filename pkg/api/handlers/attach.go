package handlers

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net/http"
	"sync"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// AttachContainer attaches to a container via HTTP hijack.
func AttachContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	name := getName(r)
	c, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	// Does the container have a terminal? Determines how we handle the
	// hijack.
	// TODO: is false a sane default?
	isTerminal := false
	spec := c.Spec()
	if spec.Process != nil {
		isTerminal = spec.Process.Terminal
	}

	// Hijack the connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		utils.InternalServerError(w, errors.Errorf("unable to hijack connection"))
		return
	}
	connection, buffer, err := hijacker.Hijack()
	if err != nil {
		utils.InternalServerError(w, errors.Wrapf(err, "error hijacking connection"))
		return
	}
	defer connection.Close()

	// TODO: What do we do about the TerminalSize channel?
	// I think we ignore it for now, but we probably need a separate API on
	// the libpod side to handle it (needs to be a separate API endpoint in
	// the remote API).

	// STDIN is literal bytes, always. So we can just pass the Reader from
	// the hijack buffer in.
	// If we are a Terminal hijack - everything is over STDOUT, so we can do
	// the same with STDOUT, though we need to wrap it to present a
	// WriteCloser instead of a writer.
	if isTerminal {
		streams := new(libpod.AttachStreams)
		streams.OutputStream = &writeCloserWrapper{buffer}
		streams.AttachOutput = true
		streams.InputStream = buffer.Reader
		streams.AttachInput = true
		// TODO: verify empty string does NOT clear detach keys.
		if err := c.Attach(streams, "", nil); err != nil {
			// TODO: How do we handle this? We can't really return
			// an HTTP status code as we've already hijacked...
			// For now, write the error and return.
			if _, err2 := buffer.Write([]byte(err.Error())); err2 != nil {
				logrus.Errorf("Error writing error %v to hijacked attach connection: %v", err, err2)
			}
		}

		// Once attach exits, the user either detached or the container
		// is stopped.
		// Return clean either way.
		return
	}

	// Non-terminal attach.
	// This is where things get Complicated.
	// STDOUT and STDERR are multiplexed over the hijacked socket.
	// This is done using an 8-byte header formatted as single-byte stream
	// type (0 STDIN (writes to STDOUT), 1 STDOUT, 2 STDERR), three 0 bytes,
	// and a 4-byte length (BE encoded) of the the following data.
	// We need to capture STDOUT and STDERR from the attach streams
	// and encode them with the appropriate header before writing.
	// TODO: We 100% don't handle STDIN right now. I need to verify but I'm
	// fairly certain that Docker broadcasts STDIN as it's written to so all
	// attach sessions get it if multiple sessions are open for the same
	// container. This will probably need Conmon changes.
	var (
		httpLock sync.Mutex
	)
	const (
		// nolint
		stdin  = 0
		stdout = 1
		stderr = 2
	)
	// This is used to indicate an error occurred somewhere during the
	// process of copying. End the attach prematurely.
	errChan := make(chan error)
	// This is used to signal a stream received EOF
	// We can use this to terminate attach.
	// TODO: When do we terminate? On receiving any EOF or do we need a
	// combination of them?
	doneChan := make(chan int)

	stdoutTransfer := new(ClosingTransferBuffer)
	stdoutTransfer.stdStream = stdout
	stdoutTransfer.httpWriter = buffer
	stdoutTransfer.httpLock = &httpLock
	stdoutTransfer.eofChan = doneChan
	stdoutTransfer.errChan = errChan

	stderrTransfer := new(ClosingTransferBuffer)
	stderrTransfer.stdStream = stderr
	stderrTransfer.httpWriter = buffer
	stderrTransfer.httpLock = &httpLock
	stderrTransfer.eofChan = doneChan
	stderrTransfer.errChan = errChan

	// We need 3 goroutines
	// One for STDOUT, one for STDERR, one for Attach itself.
	// We exit if any one of these finishes or errors.
	go stdoutTransfer.Transfer()
	go stderrTransfer.Transfer()
	go func() {
		streams := new(libpod.AttachStreams)
		// TODO: Docker allows configuration of what to attach. Right
		// now, we do not. Resolve this.
		streams.OutputStream = stdoutTransfer
		streams.ErrorStream = stderrTransfer
		streams.InputStream = buffer.Reader
		streams.AttachOutput = true
		streams.AttachError = true
		streams.AttachInput = true

		// TODO verify that "" does not clear detach keys.
		err := c.Attach(streams, "", nil)
		if err != nil {
			errChan <- err
		}
		// HACK HACK HACK
		// Send an invalid stream type over done to indicate attach
		// finished normally.
		// TODO: Find another way to do this - a Done channel maybe.
		doneChan <- 3
	}()

	// Wait for something from one of our channels.
	select {
	case doneStream := <-doneChan:
		logrus.Debugf("EOF on stream %d", doneStream)
		// TODO: We should PROBABLY not finish the attach here on just
		// an EOF
	case err := <-errChan:
		logrus.Errorf("Error while attaching to container %s: %v", c.ID(), err)
		// TODO, again: error handling - how?
		if _, err2 := buffer.Write([]byte(err.Error())); err2 != nil {
			logrus.Errorf("Error writing error %v to hijacked attach connection: %v", err, err2)
		}
	}

	// TODO: We need a way to terminate attach early when this happens.
	// It might still be running if it was an error out of Transfer().
	// We can kill stdout and stderr transfers at least.
	_ = stdoutTransfer.Close()
	_ = stderrTransfer.Close()
}

// Wraps a bufio.Writer to provide a Close() method
type writeCloserWrapper struct {
	w *bufio.ReadWriter
}

func (wrapper *writeCloserWrapper) Write(in []byte) (int, error) {
	return wrapper.w.Write(in)
}

func (wrapper *writeCloserWrapper) Close() error {
	return wrapper.w.Flush()
}

// A ClosingTransferBuffer is used to shuffle bytes from the container's STDOUT
// and STDERR to the hijacked HTTP connection.
type ClosingTransferBuffer struct {
	// What stream we are responsible for
	stdStream int
	// The buffer the container's Attach API will write to.
	containerBuff bytes.Buffer
	// An intermediate buffer. We read from containerBuff into here and then
	// append a header before writing to the HTTP connection.
	intermediateBuff []byte
	// httpWriter writes to the hijacked HTTP session.
	httpWriter io.Writer
	// httpLock ensures that STDOUT and STDERR don't try and write to the
	// HTTP writer simultaneously.
	httpLock *sync.Mutex
	// Whether we got EOF. If we did, refuse future writes.
	gotEOF bool
	// A channel to inform the main function of EOF
	eofChan chan int
	// A channel to inform the main function of errors
	errChan chan error
}

// Write is used by the container's Attach API to write to the internal buffer
// for the container's stream.
func (buf *ClosingTransferBuffer) Write(in []byte) (int, error) {
	if buf.gotEOF {
		return -1, errors.Errorf("stream is closed")
	}
	return buf.containerBuff.Write(in)
}

// Close indicates that an EOF has occurred.
func (buf *ClosingTransferBuffer) Close() error {
	if buf.gotEOF {
		return errors.Errorf("already got EOF")
	}
	buf.eofChan <- buf.stdStream
	buf.gotEOF = true

	return nil
}

// Transfer transfers contents from the container's standard stream to the
// hijacked HTTP session.
// Meant to be run as a goroutine. Reports errors via errChan in the
// ClosingTransferBuffer struct
func (buf *ClosingTransferBuffer) Transfer() {
	// Hardcoding to 1kb because this seems like a sensible number.
	// TODO: Do this in a more sane fashion.
	buf.intermediateBuff = make([]byte, 0, 1024)
	for {
		// If we got an EOF on this stream, stop transferring, but make
		// sure to drain the rest of the buffer.
		if buf.gotEOF && buf.containerBuff.Len() == 0 {
			return
		}
		numRead, err := buf.containerBuff.Read(buf.intermediateBuff)
		if err != nil {
			if err == io.EOF {
				buf.eofChan <- buf.stdStream
				buf.gotEOF = true
				return
			}
			buf.errChan <- err
			return
		}
		// Grab the HTTP lock and write the header
		buf.httpLock.Lock()
		headerBytes := []byte{byte(buf.stdStream), 0, 0, 0}
		lengthBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(lengthBytes, uint32(numRead))
		_, err = buf.httpWriter.Write(headerBytes)
		if err != nil {
			buf.errChan <- err
			buf.httpLock.Unlock()
			return
		}
		_, err = buf.httpWriter.Write(lengthBytes)
		if err != nil {
			buf.errChan <- err
			buf.httpLock.Unlock()
			return
		}
		// Header's done, write whatever we read
		_, err = buf.httpWriter.Write(buf.intermediateBuff[:(numRead - 1)])
		if err != nil {
			buf.errChan <- err
			buf.httpLock.Unlock()
			return
		}
		buf.httpLock.Unlock()
	}
}
