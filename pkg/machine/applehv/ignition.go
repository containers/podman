//go:build darwin && arm64
// +build darwin,arm64

package applehv

import (
	"net"
	"net/http"

	"github.com/containers/podman/v4/pkg/machine"
	"github.com/sirupsen/logrus"
)

// serveIgnitionOverSock allows podman to open a small httpd instance on the vsock between the host
// and guest to inject the ignitionfile into fcos
func (m *MacMachine) serveIgnitionOverSock(ignitionSocket *machine.VMFile) error {
	logrus.Debugf("reading ignition file: %s", m.IgnitionFile.GetPath())
	ignFile, err := m.IgnitionFile.Read()
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write(ignFile)
		if err != nil {
			logrus.Error("failed to serve ignition file: %v", err)
		}
	})
	listener, err := net.Listen("unix", ignitionSocket.GetPath())
	if err != nil {
		return err
	}
	logrus.Debugf("ignition socket device: %s", ignitionSocket.GetPath())
	defer func() {
		if err := listener.Close(); err != nil {
			logrus.Error(err)
		}
	}()
	return http.Serve(listener, mux)
}
