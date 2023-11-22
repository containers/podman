package sockets

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"path/filepath"
	"time"

	"github.com/containers/podman/v4/pkg/machine/define"
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
		errChan <- err
		return
	}
	_, err = bufio.NewReader(conn).ReadString('\n')

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
