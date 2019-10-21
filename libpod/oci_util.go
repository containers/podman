package libpod

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/containers/libpod/libpod/define"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Timeout before declaring that runtime has failed to kill a given
// container
const killContainerTimeout = 5 * time.Second

// ociError is used to parse the OCI runtime JSON log.  It is not part of the
// OCI runtime specifications, it follows what runc does
type ociError struct {
	Level string `json:"level,omitempty"`
	Time  string `json:"time,omitempty"`
	Msg   string `json:"msg,omitempty"`
}

// Create systemd unit name for cgroup scopes
func createUnitName(prefix string, name string) string {
	return fmt.Sprintf("%s-%s.scope", prefix, name)
}

// Bind ports to keep them closed on the host
func bindPorts(ports []ocicni.PortMapping) ([]*os.File, error) {
	var files []*os.File
	notifySCTP := false
	for _, i := range ports {
		switch i.Protocol {
		case "udp":
			addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", i.HostIP, i.HostPort))
			if err != nil {
				return nil, errors.Wrapf(err, "cannot resolve the UDP address")
			}

			server, err := net.ListenUDP("udp", addr)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot listen on the UDP port")
			}
			f, err := server.File()
			if err != nil {
				return nil, errors.Wrapf(err, "cannot get file for UDP socket")
			}
			files = append(files, f)

		case "tcp":
			addr, err := net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%d", i.HostIP, i.HostPort))
			if err != nil {
				return nil, errors.Wrapf(err, "cannot resolve the TCP address")
			}

			server, err := net.ListenTCP("tcp4", addr)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot listen on the TCP port")
			}
			f, err := server.File()
			if err != nil {
				return nil, errors.Wrapf(err, "cannot get file for TCP socket")
			}
			files = append(files, f)
		case "sctp":
			if !notifySCTP {
				notifySCTP = true
				logrus.Warnf("port reservation for SCTP is not supported")
			}
		default:
			return nil, fmt.Errorf("unknown protocol %s", i.Protocol)

		}
	}
	return files, nil
}

func getOCIRuntimeError(runtimeMsg string) error {
	r := strings.ToLower(runtimeMsg)
	if match, _ := regexp.MatchString(".*permission denied.*|.*operation not permitted.*", r); match {
		return errors.Wrapf(define.ErrOCIRuntimePermissionDenied, "%s", strings.Trim(runtimeMsg, "\n"))
	}
	if match, _ := regexp.MatchString(".*executable file not found in.*|.*no such file or directory.*", r); match {
		return errors.Wrapf(define.ErrOCIRuntimeNotFound, "%s", strings.Trim(runtimeMsg, "\n"))
	}
	return errors.Wrapf(define.ErrOCIRuntime, "%s", strings.Trim(runtimeMsg, "\n"))
}
