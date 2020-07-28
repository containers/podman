package libpod

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/containers/podman/v2/libpod/define"
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
		isV6 := net.ParseIP(i.HostIP).To4() == nil
		if i.HostIP == "" {
			isV6 = false
		}
		switch i.Protocol {
		case "udp":
			var (
				addr *net.UDPAddr
				err  error
			)
			if isV6 {
				addr, err = net.ResolveUDPAddr("udp6", fmt.Sprintf("[%s]:%d", i.HostIP, i.HostPort))
			} else {
				addr, err = net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", i.HostIP, i.HostPort))
			}
			if err != nil {
				return nil, errors.Wrapf(err, "cannot resolve the UDP address")
			}

			proto := "udp4"
			if isV6 {
				proto = "udp6"
			}
			server, err := net.ListenUDP(proto, addr)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot listen on the UDP port")
			}
			f, err := server.File()
			if err != nil {
				return nil, errors.Wrapf(err, "cannot get file for UDP socket")
			}
			files = append(files, f)

		case "tcp":
			var (
				addr *net.TCPAddr
				err  error
			)
			if isV6 {
				addr, err = net.ResolveTCPAddr("tcp6", fmt.Sprintf("[%s]:%d", i.HostIP, i.HostPort))
			} else {
				addr, err = net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%d", i.HostIP, i.HostPort))
			}
			if err != nil {
				return nil, errors.Wrapf(err, "cannot resolve the TCP address")
			}

			proto := "tcp4"
			if isV6 {
				proto = "tcp6"
			}
			server, err := net.ListenTCP(proto, addr)
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
	includeFullOutput := logrus.GetLevel() == logrus.DebugLevel

	if match := regexp.MustCompile("(?i).*permission denied.*|.*operation not permitted.*").FindString(runtimeMsg); match != "" {
		errStr := match
		if includeFullOutput {
			errStr = runtimeMsg
		}
		return errors.Wrapf(define.ErrOCIRuntimePermissionDenied, "%s", strings.Trim(errStr, "\n"))
	}
	if match := regexp.MustCompile("(?i).*executable file not found in.*|.*no such file or directory.*").FindString(runtimeMsg); match != "" {
		errStr := match
		if includeFullOutput {
			errStr = runtimeMsg
		}
		return errors.Wrapf(define.ErrOCIRuntimeNotFound, "%s", strings.Trim(errStr, "\n"))
	}
	return errors.Wrapf(define.ErrOCIRuntime, "%s", strings.Trim(runtimeMsg, "\n"))
}
