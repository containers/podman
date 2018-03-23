package libpod

import (
	"fmt"
	"io"
	"net"
	"os"
	gosignal "os/signal"
	"path/filepath"

	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/term"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/pkg/kubeutils"
	"github.com/projectatomic/libpod/utils"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/sys/unix"
	"k8s.io/client-go/tools/remotecommand"
)

/* Sync with stdpipe_t in conmon.c */
const (
	AttachPipeStdin  = 1
	AttachPipeStdout = 2
	AttachPipeStderr = 3
)

// resizeTty handles TTY resizing for Attach()
func resizeTty(resize chan remotecommand.TerminalSize) {
	sigchan := make(chan os.Signal, 1)
	gosignal.Notify(sigchan, signal.SIGWINCH)
	sendUpdate := func() {
		winsize, err := term.GetWinsize(os.Stdin.Fd())
		if err != nil {
			logrus.Warnf("Could not get terminal size %v", err)
			return
		}
		resize <- remotecommand.TerminalSize{
			Width:  winsize.Width,
			Height: winsize.Height,
		}
	}
	go func() {
		defer close(resize)
		// Update the terminal size immediately without waiting
		// for a SIGWINCH to get the correct initial size.
		sendUpdate()
		for range sigchan {
			sendUpdate()
		}
	}()
}

func (c *Container) attach(noStdin bool, keys string) error {
	// Check the validity of the provided keys first
	var err error
	detachKeys := []byte{}
	if len(keys) > 0 {
		detachKeys, err = term.ToBytes(keys)
		if err != nil {
			return errors.Wrapf(err, "invalid detach keys")
		}
	}

	// TODO: allow resize channel to be passed in for CRI-O use
	resize := make(chan remotecommand.TerminalSize)
	if terminal.IsTerminal(int(os.Stdin.Fd())) {
		resizeTty(resize)
	} else {
		defer close(resize)
	}

	logrus.Debugf("Attaching to container %s", c.ID())

	return c.attachContainerSocket(resize, noStdin, detachKeys)
}

// attachContainerSocket connects to the container's attach socket and deals with the IO
// TODO add a channel to allow interruptiong
func (c *Container) attachContainerSocket(resize <-chan remotecommand.TerminalSize, noStdIn bool, detachKeys []byte) error {
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
	logrus.Debug("connecting to socket ", c.attachSocketPath())

	conn, err := net.DialUnix("unixpacket", nil, &net.UnixAddr{Name: c.attachSocketPath(), Net: "unixpacket"})
	if err != nil {
		return errors.Wrapf(err, "failed to connect to container's attach socket: %v")
	}
	defer conn.Close()

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
