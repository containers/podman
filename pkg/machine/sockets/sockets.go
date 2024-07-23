package sockets

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"time"

	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/storage/pkg/fileutils"
	"github.com/sirupsen/logrus"
)

// SetSocket creates a new machine file for the socket and assigns it to
// `socketLoc`
func SetSocket(socketLoc *define.VMFile, path string, symlink *string) error {
	socket, err := define.NewMachineFile(path, symlink)
	if err != nil {
		return err
	}
	*socketLoc = *socket
	return nil
}

// ReadySocketPath returns the filepath for the ready socket
func ReadySocketPath(runtimeDir, machineName string) string {
	return filepath.Join(runtimeDir, fmt.Sprintf("%s_ready.sock", machineName))
}

// ListenAndWaitOnSocket waits for a new connection to the listener and sends
// any error back through the channel. ListenAndWaitOnSocket is intended to be
// used as a goroutine
func ListenAndWaitOnSocket(errChan chan<- error, listener net.Listener) {
	conn, err := listener.Accept()
	if err != nil {
		logrus.Debug("failed to connect to ready socket")
		errChan <- err
		return
	}
	_, err = bufio.NewReader(conn).ReadString('\n')
	logrus.Debug("ready ack received")

	if closeErr := conn.Close(); closeErr != nil {
		errChan <- closeErr
		return
	}

	errChan <- err
}

// DialSocketWithBackoffs attempts to connect to the socket in maxBackoffs attempts
func DialSocketWithBackoffs(maxBackoffs int, backoff time.Duration, socketPath string) (conn net.Conn, err error) {
	for i := 0; i < maxBackoffs; i++ {
		if i > 0 {
			time.Sleep(backoff)
			backoff *= 2
		}
		conn, err = net.Dial("unix", socketPath)
		if err == nil {
			return conn, nil
		}
	}
	return nil, err
}

// DialSocketWithBackoffsAndProcCheck attempts to connect to the socket in
// maxBackoffs attempts. After every failure to connect, it makes sure the
// specified process is alive
func DialSocketWithBackoffsAndProcCheck(
	maxBackoffs int,
	backoff time.Duration,
	socketPath string,
	checkProccessStatus func(string, int, *bytes.Buffer) error,
	procHint string,
	procPid int,
	errBuf *bytes.Buffer,
) (conn net.Conn, err error) {
	for i := 0; i < maxBackoffs; i++ {
		if i > 0 {
			time.Sleep(backoff)
			backoff *= 2
		}
		conn, err = net.Dial("unix", socketPath)
		if err == nil {
			return conn, nil
		}

		// check to make sure process denoted by procHint is alive
		err = checkProccessStatus(procHint, procPid, errBuf)
		if err != nil {
			return nil, err
		}
	}
	return nil, err
}

// WaitForSocketWithBackoffs attempts to discover listening socket in maxBackoffs attempts
func WaitForSocketWithBackoffs(maxBackoffs int, backoff time.Duration, socketPath string, name string) error {
	backoffWait := backoff
	logrus.Debugf("checking that %q socket is ready", name)
	for i := 0; i < maxBackoffs; i++ {
		err := fileutils.Exists(socketPath)
		if err == nil {
			return nil
		}
		time.Sleep(backoffWait)
		backoffWait *= 2
	}
	return fmt.Errorf("unable to connect to %q socket at %q", name, socketPath)
}

// ToUnixURL converts `socketLoc` into URL representation
func ToUnixURL(socketLoc *define.VMFile) (*url.URL, error) {
	p := socketLoc.GetPath()
	if !filepath.IsAbs(p) {
		return nil, fmt.Errorf("socket path must be absolute %q", p)
	}
	s, err := url.Parse("unix:///")
	if err != nil {
		return nil, err
	}
	s = s.JoinPath(filepath.ToSlash(p))
	return s, nil
}
