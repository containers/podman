//+build linux

package libpod

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/errorhandling"
	"github.com/containers/podman/v4/pkg/kubeutils"
	"github.com/containers/podman/v4/utils"
	"github.com/moby/term"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

/* Sync with stdpipe_t in conmon.c */
const (
	AttachPipeStdin  = 1
	AttachPipeStdout = 2
	AttachPipeStderr = 3
)

func openUnixSocket(path string) (*net.UnixConn, error) {
	fd, err := unix.Open(path, unix.O_PATH, 0)
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)
	return net.DialUnix("unixpacket", nil, &net.UnixAddr{Name: fmt.Sprintf("/proc/self/fd/%d", fd), Net: "unixpacket"})
}

// Attach to the given container
// Does not check if state is appropriate
// started is only required if startContainer is true
func (c *Container) attach(streams *define.AttachStreams, keys string, resize <-chan define.TerminalSize, startContainer bool, started chan bool, attachRdy chan<- bool) error {
	passthrough := c.LogDriver() == define.PassthroughLogging

	if !streams.AttachOutput && !streams.AttachError && !streams.AttachInput && !passthrough {
		return errors.Wrapf(define.ErrInvalidArg, "must provide at least one stream to attach to")
	}
	if startContainer && started == nil {
		return errors.Wrapf(define.ErrInternal, "started chan not passed when startContainer set")
	}

	detachKeys, err := processDetachKeys(keys)
	if err != nil {
		return err
	}

	var conn *net.UnixConn
	if !passthrough {
		logrus.Debugf("Attaching to container %s", c.ID())

		registerResizeFunc(resize, c.bundlePath())

		attachSock, err := c.AttachSocketPath()
		if err != nil {
			return err
		}

		conn, err = openUnixSocket(attachSock)
		if err != nil {
			return errors.Wrapf(err, "failed to connect to container's attach socket: %v", attachSock)
		}
		defer func() {
			if err := conn.Close(); err != nil {
				logrus.Errorf("unable to close socket: %q", err)
			}
		}()
	}

	// If starting was requested, start the container and notify when that's
	// done.
	if startContainer {
		if err := c.start(); err != nil {
			return err
		}
		started <- true
	}

	if passthrough {
		return nil
	}

	receiveStdoutError, stdinDone := setupStdioChannels(streams, conn, detachKeys)
	if attachRdy != nil {
		attachRdy <- true
	}
	return readStdio(conn, streams, receiveStdoutError, stdinDone)
}

// Attach to the given container's exec session
// attachFd and startFd must be open file descriptors
// attachFd must be the output side of the fd. attachFd is used for two things:
//  conmon will first send a nonce value across the pipe indicating it has set up its side of the console socket
//    this ensures attachToExec gets all of the output of the called process
//  conmon will then send the exit code of the exec process, or an error in the exec session
// startFd must be the input side of the fd.
// newSize resizes the tty to this size before the process is started, must be nil if the exec session has no tty
//   conmon will wait to start the exec session until the parent process has setup the console socket.
//   Once attachToExec successfully attaches to the console socket, the child conmon process responsible for calling runtime exec
//     will read from the output side of start fd, thus learning to start the child process.
// Thus, the order goes as follow:
// 1. conmon parent process sets up its console socket. sends on attachFd
// 2. attachToExec attaches to the console socket after reading on attachFd and resizes the tty
// 3. child waits on startFd for attachToExec to attach to said console socket
// 4. attachToExec sends on startFd, signalling it has attached to the socket and child is ready to go
// 5. child receives on startFd, runs the runtime exec command
// attachToExec is responsible for closing startFd and attachFd
func (c *Container) attachToExec(streams *define.AttachStreams, keys *string, sessionID string, startFd, attachFd *os.File, newSize *define.TerminalSize) error {
	if !streams.AttachOutput && !streams.AttachError && !streams.AttachInput {
		return errors.Wrapf(define.ErrInvalidArg, "must provide at least one stream to attach to")
	}
	if startFd == nil || attachFd == nil {
		return errors.Wrapf(define.ErrInvalidArg, "start sync pipe and attach sync pipe must be defined for exec attach")
	}

	defer errorhandling.CloseQuiet(startFd)
	defer errorhandling.CloseQuiet(attachFd)

	detachString := config.DefaultDetachKeys
	if keys != nil {
		detachString = *keys
	}
	detachKeys, err := processDetachKeys(detachString)
	if err != nil {
		return err
	}

	logrus.Debugf("Attaching to container %s exec session %s", c.ID(), sessionID)

	// set up the socket path, such that it is the correct length and location for exec
	sockPath, err := c.execAttachSocketPath(sessionID)
	if err != nil {
		return err
	}

	// 2: read from attachFd that the parent process has set up the console socket
	if _, err := readConmonPipeData(c.ociRuntime.Name(), attachFd, ""); err != nil {
		return err
	}

	// resize before we start the container process
	if newSize != nil {
		err = c.ociRuntime.ExecAttachResize(c, sessionID, *newSize)
		if err != nil {
			logrus.Warnf("Resize failed: %v", err)
		}
	}

	// 2: then attach
	conn, err := openUnixSocket(sockPath)
	if err != nil {
		return errors.Wrapf(err, "failed to connect to container's attach socket: %v", sockPath)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			logrus.Errorf("Unable to close socket: %q", err)
		}
	}()

	// start listening on stdio of the process
	receiveStdoutError, stdinDone := setupStdioChannels(streams, conn, detachKeys)

	// 4: send start message to child
	if err := writeConmonPipeData(startFd); err != nil {
		return err
	}

	return readStdio(conn, streams, receiveStdoutError, stdinDone)
}

func processDetachKeys(keys string) ([]byte, error) {
	// Check the validity of the provided keys first
	if len(keys) == 0 {
		return []byte{}, nil
	}
	detachKeys, err := term.ToBytes(keys)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid detach keys")
	}
	return detachKeys, nil
}

func registerResizeFunc(resize <-chan define.TerminalSize, bundlePath string) {
	kubeutils.HandleResizing(resize, func(size define.TerminalSize) {
		controlPath := filepath.Join(bundlePath, "ctl")
		controlFile, err := os.OpenFile(controlPath, unix.O_WRONLY, 0)
		if err != nil {
			logrus.Debugf("Could not open ctl file: %v", err)
			return
		}
		defer controlFile.Close()

		logrus.Debugf("Received a resize event: %+v", size)
		if _, err = fmt.Fprintf(controlFile, "%d %d %d\n", 1, size.Height, size.Width); err != nil {
			logrus.Warnf("Failed to write to control file to resize terminal: %v", err)
		}
	})
}

func setupStdioChannels(streams *define.AttachStreams, conn *net.UnixConn, detachKeys []byte) (chan error, chan error) {
	receiveStdoutError := make(chan error)
	go func() {
		receiveStdoutError <- redirectResponseToOutputStreams(streams.OutputStream, streams.ErrorStream, streams.AttachOutput, streams.AttachError, conn)
	}()

	stdinDone := make(chan error)
	go func() {
		var err error
		if streams.AttachInput {
			_, err = utils.CopyDetachable(conn, streams.InputStream, detachKeys)
		}
		stdinDone <- err
	}()

	return receiveStdoutError, stdinDone
}

func redirectResponseToOutputStreams(outputStream, errorStream io.Writer, writeOutput, writeError bool, conn io.Reader) error {
	var err error
	buf := make([]byte, 8192+1) /* Sync with conmon STDIO_BUF_SIZE */
	for {
		nr, er := conn.Read(buf)
		if nr > 0 {
			var dst io.Writer
			var doWrite bool
			switch buf[0] {
			case AttachPipeStdout:
				dst = outputStream
				doWrite = writeOutput
			case AttachPipeStderr:
				dst = errorStream
				doWrite = writeError
			default:
				logrus.Infof("Received unexpected attach type %+d", buf[0])
			}
			if dst == nil {
				return errors.New("output destination cannot be nil")
			}

			if doWrite {
				nw, ew := dst.Write(buf[1:nr])
				if ew != nil {
					err = ew
					break
				}
				if nr != nw+1 {
					err = io.ErrShortWrite
					break
				}
			}
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			err = er
			break
		}
	}
	return err
}

func readStdio(conn *net.UnixConn, streams *define.AttachStreams, receiveStdoutError, stdinDone chan error) error {
	var err error
	select {
	case err = <-receiveStdoutError:
		conn.CloseWrite()
		return err
	case err = <-stdinDone:
		if err == define.ErrDetach {
			conn.CloseWrite()
			return err
		}
		if err == nil {
			// copy stdin is done, close it
			if connErr := conn.CloseWrite(); connErr != nil {
				logrus.Errorf("Unable to close conn: %v", connErr)
			}
		}
		if streams.AttachOutput || streams.AttachError {
			return <-receiveStdoutError
		}
	}
	return nil
}
