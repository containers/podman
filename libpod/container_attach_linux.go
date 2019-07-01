//+build linux

package libpod

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/kubeutils"
	"github.com/containers/libpod/utils"
	"github.com/docker/docker/pkg/term"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"k8s.io/client-go/tools/remotecommand"
)

//#include <sys/un.h>
// extern int unix_path_length(){struct sockaddr_un addr; return sizeof(addr.sun_path) - 1;}
import "C"

/* Sync with stdpipe_t in conmon.c */
const (
	AttachPipeStdin  = 1
	AttachPipeStdout = 2
	AttachPipeStderr = 3
)

// Attach to the given container
// Does not check if state is appropriate
func (c *Container) attach(streams *AttachStreams, keys string, resize <-chan remotecommand.TerminalSize, startContainer bool, started chan bool) error {
	if !streams.AttachOutput && !streams.AttachError && !streams.AttachInput {
		return errors.Wrapf(define.ErrInvalidArg, "must provide at least one stream to attach to")
	}

	logrus.Debugf("Attaching to container %s", c.ID())

	return c.attachContainerSocket(resize, keys, streams, startContainer, started)
}

// attachContainerSocket connects to the container's attach socket and deals with the IO.
// started is only required if startContainer is true
// TODO add a channel to allow interrupting
func (c *Container) attachContainerSocket(resize <-chan remotecommand.TerminalSize, keys string, streams *AttachStreams, startContainer bool, started chan bool) error {
	if startContainer && started == nil {
		return errors.Wrapf(define.ErrInternal, "started chan not passed when startContainer set")
	}

	// Use default detach keys when keys aren't passed or specified in libpod.conf
	if len(keys) == 0 {
		keys = DefaultDetachKeys
	}

	// Check the validity of the provided keys
	detachKeys := []byte{}
	var err error
	detachKeys, err = term.ToBytes(keys)
	if err != nil {
		return errors.Wrapf(err, "invalid detach keys")
	}

	kubeutils.HandleResizing(resize, func(size remotecommand.TerminalSize) {
		controlPath := filepath.Join(c.bundlePath(), "ctl")
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

	socketPath := c.AttachSocketPath()

	maxUnixLength := int(C.unix_path_length())
	if maxUnixLength < len(socketPath) {
		socketPath = socketPath[0:maxUnixLength]
	}

	logrus.Debug("connecting to socket ", socketPath)

	conn, err := net.DialUnix("unixpacket", nil, &net.UnixAddr{Name: socketPath, Net: "unixpacket"})
	if err != nil {
		return errors.Wrapf(err, "failed to connect to container's attach socket: %v", socketPath)
	}
	defer conn.Close()

	// If starting was requested, start the container and notify when that's
	// done.
	if startContainer {
		if err := c.start(); err != nil {
			return err
		}
		started <- true
	}

	receiveStdoutError := make(chan error)
	go func() {
		receiveStdoutError <- redirectResponseToOutputStreams(streams.OutputStream, streams.ErrorStream, streams.AttachOutput, streams.AttachError, conn)
	}()

	stdinDone := make(chan error)
	go func() {
		var err error
		if streams.AttachInput {
			_, err = utils.CopyDetachable(conn, streams.InputStream, detachKeys)
			conn.CloseWrite()
		}
		stdinDone <- err
	}()

	select {
	case err := <-receiveStdoutError:
		return err
	case err := <-stdinDone:
		if err == define.ErrDetach {
			return err
		}
		if streams.AttachOutput || streams.AttachError {
			return <-receiveStdoutError
		}
	}
	return nil
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
