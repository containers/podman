package libpod

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/containers/common/libnetwork/types"
	"github.com/sirupsen/logrus"
)

const machineGvproxyEndpoint = "gateway.containers.internal"

// machineExpose is the struct for the gvproxy port forwarding api send via json
type machineExpose struct {
	// Local is the local address on the vm host, format is ip:port
	Local string `json:"local"`
	// Remote is used to specify the vm ip:port
	Remote string `json:"remote,omitempty"`
	// Protocol to forward, tcp or udp
	Protocol string `json:"protocol"`
}

func requestMachinePorts(expose bool, ports []types.PortMapping) error {
	url := "http://" + machineGvproxyEndpoint + "/services/forwarder/"
	if expose {
		url = url + "expose"
	} else {
		url = url + "unexpose"
	}
	ctx := context.Background()
	client := &http.Client{}
	buf := new(bytes.Buffer)
	for num, port := range ports {
		protocols := strings.Split(port.Protocol, ",")
		for _, protocol := range protocols {
			for i := uint16(0); i < port.Range; i++ {
				machinePort := machineExpose{
					Local:    net.JoinHostPort(port.HostIP, strconv.FormatInt(int64(port.HostPort+i), 10)),
					Protocol: protocol,
				}
				if expose {
					// only set the remote port the ip will be automatically be set by gvproxy
					machinePort.Remote = ":" + strconv.FormatInt(int64(port.HostPort+i), 10)
				}

				// post request
				if err := json.NewEncoder(buf).Encode(machinePort); err != nil {
					if expose {
						// in case of an error make sure to unexpose the other ports
						if cerr := requestMachinePorts(false, ports[:num]); cerr != nil {
							logrus.Errorf("failed to free gvproxy machine ports: %v", cerr)
						}
					}
					return err
				}
				if err := makeMachineRequest(ctx, client, url, buf); err != nil {
					if expose {
						// in case of an error make sure to unexpose the other ports
						if cerr := requestMachinePorts(false, ports[:num]); cerr != nil {
							logrus.Errorf("failed to free gvproxy machine ports: %v", cerr)
						}
					}
					return err
				}
				buf.Reset()
			}
		}
	}
	return nil
}

func makeMachineRequest(ctx context.Context, client *http.Client, url string, buf io.Reader) error {
	//var buf io.ReadWriter
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, buf)
	if err != nil {
		return err
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return annotateGvproxyResponseError(resp.Body)
	}
	return nil
}

func annotateGvproxyResponseError(r io.Reader) error {
	b, err := ioutil.ReadAll(r)
	if err == nil && len(b) > 0 {
		return fmt.Errorf("something went wrong with the request: %q", string(b))
	}
	return errors.New("something went wrong with the request, could not read response")
}

// exposeMachinePorts exposes the ports for podman machine via gvproxy
func (r *Runtime) exposeMachinePorts(ports []types.PortMapping) error {
	if !r.config.Engine.MachineEnabled {
		return nil
	}
	return requestMachinePorts(true, ports)
}

// unexposeMachinePorts closes the ports for podman machine via gvproxy
func (r *Runtime) unexposeMachinePorts(ports []types.PortMapping) error {
	if !r.config.Engine.MachineEnabled {
		return nil
	}
	return requestMachinePorts(false, ports)
}
