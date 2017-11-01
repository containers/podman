package server

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/docker/docker/pkg/pools"
	"github.com/kubernetes-incubator/cri-o/oci"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// PortForward prepares a streaming endpoint to forward ports from a PodSandbox.
func (s *Server) PortForward(ctx context.Context, req *pb.PortForwardRequest) (*pb.PortForwardResponse, error) {
	logrus.Debugf("PortForwardRequest %+v", req)

	resp, err := s.GetPortForward(req)

	if err != nil {
		return nil, fmt.Errorf("unable to prepare portforward endpoint")
	}

	return resp, nil
}

func (ss streamService) PortForward(podSandboxID string, port int32, stream io.ReadWriteCloser) error {
	c := ss.runtimeServer.GetSandboxContainer(podSandboxID)

	if c == nil {
		return fmt.Errorf("could not find container for sandbox %q", podSandboxID)
	}

	if err := ss.runtimeServer.Runtime().UpdateStatus(c); err != nil {
		return err
	}

	cState := ss.runtimeServer.Runtime().ContainerStatus(c)
	if !(cState.Status == oci.ContainerStateRunning || cState.Status == oci.ContainerStateCreated) {
		return fmt.Errorf("container is not created or running")
	}

	containerPid := cState.Pid
	socatPath, lookupErr := exec.LookPath("socat")
	if lookupErr != nil {
		return fmt.Errorf("unable to do port forwarding: socat not found")
	}

	args := []string{"-t", fmt.Sprintf("%d", containerPid), "-n", socatPath, "-", fmt.Sprintf("TCP4:localhost:%d", port)}

	nsenterPath, lookupErr := exec.LookPath("nsenter")
	if lookupErr != nil {
		return fmt.Errorf("unable to do port forwarding: nsenter not found")
	}

	commandString := fmt.Sprintf("%s %s", nsenterPath, strings.Join(args, " "))
	logrus.Debugf("executing port forwarding command: %s", commandString)

	command := exec.Command(nsenterPath, args...)
	command.Stdout = stream

	stderr := new(bytes.Buffer)
	command.Stderr = stderr

	// If we use Stdin, command.Run() won't return until the goroutine that's copying
	// from stream finishes. Unfortunately, if you have a client like telnet connected
	// via port forwarding, as long as the user's telnet client is connected to the user's
	// local listener that port forwarding sets up, the telnet session never exits. This
	// means that even if socat has finished running, command.Run() won't ever return
	// (because the client still has the connection and stream open).
	//
	// The work around is to use StdinPipe(), as Wait() (called by Run()) closes the pipe
	// when the command (socat) exits.
	inPipe, err := command.StdinPipe()
	if err != nil {
		return fmt.Errorf("unable to do port forwarding: error creating stdin pipe: %v", err)
	}
	go func() {
		pools.Copy(inPipe, stream)
		inPipe.Close()
	}()

	if err := command.Run(); err != nil {
		return fmt.Errorf("%v: %s", err, stderr.String())
	}

	return nil
}
