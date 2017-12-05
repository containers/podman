package libpod

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/term"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/utils"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/sys/unix"
	"k8s.io/client-go/tools/remotecommand"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
)

/* Sync with stdpipe_t in conmon.c */
const (
	AttachPipeStdin  = 1
	AttachPipeStdout = 2
	AttachPipeStderr = 3
)

// attachContainerSocket connects to the container's attach socket and deals with the IO
func (c *Container) attachContainerSocket(resize <-chan remotecommand.TerminalSize, noStdIn bool, detachKeys []byte, attached chan<- bool) error {
	inputStream := os.Stdin
	outputStream := os.Stdout
	errorStream := os.Stderr
	defer inputStream.Close()

	// TODO Renable this when tty/terminal discussion is had.
	/*
		tty, err := strconv.ParseBool(c.runningSpec.Annotations["io.kubernetes.cri-o.TTY"])
		if err != nil {
			return errors.Wrapf(err, "unable to parse annotations in %s", c.ID)
		}
		if !tty {
			return errors.Errorf("no tty available for %s", c.ID())
		}
	*/

	if terminal.IsTerminal(int(inputStream.Fd())) {
		oldTermState, err := term.SaveState(inputStream.Fd())
		if err != nil {
			return errors.Wrapf(err, "unable to save terminal state")
		}

		defer term.RestoreTerminal(inputStream.Fd(), oldTermState)
	}

	// Put both input and output into raw
	if !noStdIn {
		term.SetRawTerminal(inputStream.Fd())
	}

	controlPath := filepath.Join(c.bundlePath(), "ctl")
	controlFile, err := os.OpenFile(controlPath, unix.O_WRONLY, 0)
	if err != nil {
		return errors.Wrapf(err, "failed to open container ctl file: %v")
	}

	kubecontainer.HandleResizing(resize, func(size remotecommand.TerminalSize) {
		logrus.Debugf("Received a resize event: %+v", size)
		_, err := fmt.Fprintf(controlFile, "%d %d %d\n", 1, size.Height, size.Width)
		if err != nil {
			logrus.Warnf("Failed to write to control file to resize terminal: %v", err)
		}
	})
	logrus.Debug("connecting to socket ", c.attachSocketPath())

	conn, err := net.DialUnix("unixpacket", nil, &net.UnixAddr{Name: c.attachSocketPath(), Net: "unixpacket"})
	if err != nil {
		return errors.Wrapf(err, "failed to connect to container's attach socket: %v")
	}
	defer conn.Close()

	// signal back that the connection was made
	attached <- true

	receiveStdoutError := make(chan error)
	if outputStream != nil || errorStream != nil {
		go func() {
			receiveStdoutError <- redirectResponseToOutputStreams(outputStream, errorStream, conn)
		}()
	}

	stdinDone := make(chan error)
	go func() {
		var err error
		if inputStream != nil && !noStdIn {
			_, err = utils.CopyDetachable(conn, inputStream, detachKeys)
			conn.CloseWrite()
		}
		stdinDone <- err
	}()

	select {
	case err := <-receiveStdoutError:
		return err
	case err := <-stdinDone:
		if _, ok := err.(utils.DetachError); ok {
			return nil
		}
		if outputStream != nil || errorStream != nil {
			return <-receiveStdoutError
		}
	}
	return nil
}

func redirectResponseToOutputStreams(outputStream, errorStream io.Writer, conn io.Reader) error {
	var err error
	buf := make([]byte, 8192+1) /* Sync with conmon STDIO_BUF_SIZE */
	for {
		nr, er := conn.Read(buf)
		if nr > 0 {
			var dst io.Writer
			switch buf[0] {
			case AttachPipeStdout:
				dst = outputStream
			case AttachPipeStderr:
				dst = errorStream
			default:
				logrus.Infof("Received unexpected attach type %+d", buf[0])
			}

			if dst != nil {
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
