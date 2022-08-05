package notifyproxy

import (
	"io/ioutil"
	"net"
	"os"
	"strings"
	"syscall"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/sirupsen/logrus"
)

// SendMessage sends the specified message to the specified socket.
func SendMessage(socketPath string, message string) error {
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
}

// New creates a NotifyProxy.  The specified temp directory can be left empty.
func New(tmpDir string) (*NotifyProxy, error) {
	tempFile, err := ioutil.TempFile(tmpDir, "-podman-notify-proxy.sock")
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

// WaitAndClose waits until receiving the `READY` notify message and close the
// listener. Note that the this function must only be executed inside a systemd
// service which will kill the process after a given timeout.
func (p *NotifyProxy) WaitAndClose() error {
	defer func() {
		if err := p.close(); err != nil {
			logrus.Errorf("Closing notify proxy: %v", err)
		}
	}()

	for {
		buf := make([]byte, 1024)
		num, err := p.connection.Read(buf)
		if err != nil {
			return err
		}
		for _, s := range strings.Split(string(buf[:num]), "\n") {
			if s == daemon.SdNotifyReady {
				return nil
			}
		}
	}
}
