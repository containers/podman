package createconfig

import (
	"fmt"
	"net"
	"strconv"

	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ExposedPorts parses user and image ports and returns binding information
func ExposedPorts(expose, publish []string, publishAll bool, imageExposedPorts map[string]struct{}) (map[nat.Port][]nat.PortBinding, error) {
	containerPorts := make(map[string]string)

	// add expose ports from the image itself
	for expose := range imageExposedPorts {
		_, port := nat.SplitProtoPort(expose)
		containerPorts[port] = ""
	}

	// add the expose ports from the user (--expose)
	// can be single or a range
	for _, expose := range expose {
		//support two formats for expose, original format <portnum>/[<proto>] or <startport-endport>/[<proto>]
		_, port := nat.SplitProtoPort(expose)
		//parse the start and end port and create a sequence of ports to expose
		//if expose a port, the start and end port are the same
		start, end, err := nat.ParsePortRange(port)
		if err != nil {
			return nil, fmt.Errorf("invalid range format for --expose: %s, error: %s", expose, err)
		}
		for i := start; i <= end; i++ {
			containerPorts[strconv.Itoa(int(i))] = ""
		}
	}

	// parse user inputted port bindings
	pbPorts, portBindings, err := nat.ParsePortSpecs(publish)
	if err != nil {
		return nil, err
	}

	// delete exposed container ports if being used by -p
	for i := range pbPorts {
		delete(containerPorts, i.Port())
	}

	// iterate container ports and make port bindings from them
	if publishAll {
		for e := range containerPorts {
			//support two formats for expose, original format <portnum>/[<proto>] or <startport-endport>/[<proto>]
			//proto, port := nat.SplitProtoPort(e)
			p, err := nat.NewPort("tcp", e)
			if err != nil {
				return nil, err
			}
			rp, err := getRandomPort()
			if err != nil {
				return nil, err
			}
			logrus.Debug(fmt.Sprintf("Using random host port %d with container port %d", rp, p.Int()))
			portBindings[p] = CreatePortBinding(rp, "")
		}
	}

	// We need to see if any host ports are not populated and if so, we need to assign a
	// random port to them.
	for k, pb := range portBindings {
		if pb[0].HostPort == "" {
			hostPort, err := getRandomPort()
			if err != nil {
				return nil, err
			}
			logrus.Debug(fmt.Sprintf("Using random host port %d with container port %s", hostPort, k.Port()))
			pb[0].HostPort = strconv.Itoa(hostPort)
		}
	}
	return portBindings, nil
}

func getRandomPort() (int, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, errors.Wrapf(err, "unable to get free port")
	}
	defer l.Close()
	_, randomPort, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return 0, errors.Wrapf(err, "unable to determine free port")
	}
	rp, err := strconv.Atoi(randomPort)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to convert random port to int")
	}
	return rp, nil
}

//CreatePortBinding takes port (int) and IP (string) and creates an array of portbinding structs
func CreatePortBinding(hostPort int, hostIP string) []nat.PortBinding {
	pb := nat.PortBinding{
		HostPort: strconv.Itoa(hostPort),
	}
	pb.HostIP = hostIP
	return []nat.PortBinding{pb}
}
