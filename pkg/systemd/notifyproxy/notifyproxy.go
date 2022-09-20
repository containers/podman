package notifyproxy

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/sirupsen/logrus"
)

// SendMessage sends the specified message to the specified socket.
// No message is sent if no socketPath is provided and the NOTIFY_SOCKET
// variable is not set either.
func SendMessage(socketPath string, message string) error {
	if socketPath == "" {
		socketPath, _ = os.LookupEnv("NOTIFY_SOCKET")
		if socketPath == "" {
			return nil
		}
	}
	socketAddr := &net.UnixAddr{
		Name: socketPath,
		Net:  "unixgram",
	}
	conn, err := net.DialUnix(socketAddr.Net, nil, socketAddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write([]byte(message))
	return err
}

// NotifyProxy can be used to proxy notify messages.
type NotifyProxy struct {
	connection *net.UnixConn
	socketPath string
	container  Container // optional
}

// New creates a NotifyProxy.  The specified temp directory can be left empty.
func New(tmpDir string) (*NotifyProxy, error) {
	tempFile, err := os.CreateTemp(tmpDir, "-podman-notify-proxy.sock")
	if err != nil {
		return nil, err
	}
	defer tempFile.Close()

	socketPath := tempFile.Name()
	if err := syscall.Unlink(socketPath); err != nil { // Unlink the socket so we can bind it
		return nil, err
	}

	socketAddr := &net.UnixAddr{
		Name: socketPath,
		Net:  "unixgram",
	}
	conn, err := net.ListenUnixgram(socketAddr.Net, socketAddr)
	if err != nil {
		return nil, err
	}

	return &NotifyProxy{connection: conn, socketPath: socketPath}, nil
}

// SocketPath returns the path of the socket the proxy is listening on.
func (p *NotifyProxy) SocketPath() string {
	return p.socketPath
}

// close closes the listener and removes the socket.
func (p *NotifyProxy) close() error {
	defer os.Remove(p.socketPath)
	return p.connection.Close()
}

// AddContainer associates a container with the proxy.
func (p *NotifyProxy) AddContainer(container Container) {
	p.container = container
}

// ErrNoReadyMessage is returned when we are waiting for the READY message of a
// container that is not in the running state anymore.
var ErrNoReadyMessage = errors.New("container stopped running before READY message was received")

// Container avoids a circular dependency among this package and libpod.
type Container interface {
	State() (define.ContainerStatus, error)
	ID() string
}

// WaitAndClose waits until receiving the `READY` notify message and close the
// listener. Note that the this function must only be executed inside a systemd
// service which will kill the process after a given timeout.
// If the (optional) container stopped running before the `READY` is received,
// the waiting gets canceled and ErrNoReadyMessage is returned.
func (p *NotifyProxy) WaitAndClose() error {
	defer func() {
		if err := p.close(); err != nil {
			logrus.Errorf("Closing notify proxy: %v", err)
		}
	}()

	const bufferSize = 1024
	sBuilder := strings.Builder{}
	for {
		// Set a read deadline of one second such that we achieve a
		// non-blocking read and can check if the container has already
		// stopped running; in that case no READY message will be send
		// and we're done.
		if err := p.connection.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			return err
		}

		for {
			buffer := make([]byte, bufferSize)
			num, err := p.connection.Read(buffer)
			if err != nil {
				if !errors.Is(err, os.ErrDeadlineExceeded) && !errors.Is(err, io.EOF) {
					return err
				}
			}
			sBuilder.Write(buffer[:num])
			if num != bufferSize || buffer[num-1] == '\n' {
				break
			}
		}

		for _, line := range strings.Split(sBuilder.String(), "\n") {
			if line == daemon.SdNotifyReady {
				return nil
			}
		}
		sBuilder.Reset()

		if p.container == nil {
			continue
		}

		state, err := p.container.State()
		if err != nil {
			return err
		}
		if state != define.ContainerStateRunning {
			return fmt.Errorf("%w: %s", ErrNoReadyMessage, p.container.ID())
		}
	}
}
