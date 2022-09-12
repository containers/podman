//go:build linux
// +build linux

package libpod

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/psgo"
	"github.com/google/shlex"
	"github.com/sirupsen/logrus"
)

// Top gathers statistics about the running processes in a container. It returns a
// []string for output
func (c *Container) Top(descriptors []string) ([]string, error) {
	if c.config.NoCgroups {
		return nil, fmt.Errorf("cannot run top on container %s as it did not create a cgroup: %w", c.ID(), define.ErrNoCgroups)
	}

	conStat, err := c.State()
	if err != nil {
		return nil, fmt.Errorf("unable to look up state for %s: %w", c.ID(), err)
	}
	if conStat != define.ContainerStateRunning {
		return nil, errors.New("top can only be used on running containers")
	}

	// Also support comma-separated input.
	psgoDescriptors := []string{}
	for _, d := range descriptors {
		for _, s := range strings.Split(d, ",") {
			if s != "" {
				psgoDescriptors = append(psgoDescriptors, s)
			}
		}
	}

	// If we encountered an ErrUnknownDescriptor error, fallback to executing
	// ps(1). This ensures backwards compatibility to users depending on ps(1)
	// and makes sure we're ~compatible with docker.
	output, psgoErr := c.GetContainerPidInformation(psgoDescriptors)
	if psgoErr == nil {
		return output, nil
	}
	if !errors.Is(psgoErr, psgo.ErrUnknownDescriptor) {
		return nil, psgoErr
	}

	// Note that the descriptors to ps(1) must be shlexed (see #12452).
	psDescriptors := []string{}
	for _, d := range descriptors {
		shSplit, err := shlex.Split(d)
		if err != nil {
			return nil, fmt.Errorf("parsing ps args: %w", err)
		}
		for _, s := range shSplit {
			if s != "" {
				psDescriptors = append(psDescriptors, s)
			}
		}
	}

	output, err = c.execPS(psDescriptors)
	if err != nil {
		return nil, fmt.Errorf("executing ps(1) in the container: %w", err)
	}

	// Trick: filter the ps command from the output instead of
	// checking/requiring PIDs in the output.
	filtered := []string{}
	cmd := strings.Join(descriptors, " ")
	for _, line := range output {
		if !strings.Contains(line, cmd) {
			filtered = append(filtered, line)
		}
	}

	return filtered, nil
}

// GetContainerPidInformation returns process-related data of all processes in
// the container.  The output data can be controlled via the `descriptors`
// argument which expects format descriptors and supports all AIXformat
// descriptors of ps (1) plus some additional ones to for instance inspect the
// set of effective capabilities.  Each element in the returned string slice
// is a tab-separated string.
//
// For more details, please refer to github.com/containers/psgo.
func (c *Container) GetContainerPidInformation(descriptors []string) ([]string, error) {
	pid := strconv.Itoa(c.state.PID)
	// NOTE: psgo returns a [][]string to give users the ability to apply
	//       filters on the data.  We need to change the API here
	//       to return a [][]string if we want to make use of
	//       filtering.
	opts := psgo.JoinNamespaceOpts{FillMappings: rootless.IsRootless()}

	psgoOutput, err := psgo.JoinNamespaceAndProcessInfoWithOptions(pid, descriptors, &opts)
	if err != nil {
		return nil, err
	}
	res := []string{}
	for _, out := range psgoOutput {
		res = append(res, strings.Join(out, "\t"))
	}
	return res, nil
}

// execPS executes ps(1) with the specified args in the container.
func (c *Container) execPS(args []string) ([]string, error) {
	rPipe, wPipe, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer wPipe.Close()
	defer rPipe.Close()

	rErrPipe, wErrPipe, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer wErrPipe.Close()
	defer rErrPipe.Close()

	streams := new(define.AttachStreams)
	streams.OutputStream = wPipe
	streams.ErrorStream = wErrPipe
	streams.AttachOutput = true
	streams.AttachError = true

	stdout := []string{}
	go func() {
		scanner := bufio.NewScanner(rPipe)
		for scanner.Scan() {
			stdout = append(stdout, scanner.Text())
		}
	}()
	stderr := []string{}
	go func() {
		scanner := bufio.NewScanner(rErrPipe)
		for scanner.Scan() {
			stderr = append(stderr, scanner.Text())
		}
	}()

	cmd := append([]string{"ps"}, args...)
	config := new(ExecConfig)
	config.Command = cmd
	ec, err := c.Exec(config, streams, nil)
	if err != nil {
		return nil, err
	} else if ec != 0 {
		return nil, fmt.Errorf("runtime failed with exit status: %d and output: %s", ec, strings.Join(stderr, " "))
	}

	if logrus.GetLevel() >= logrus.DebugLevel {
		// If we're running in debug mode or higher, we might want to have a
		// look at stderr which includes debug logs from conmon.
		for _, log := range stderr {
			logrus.Debugf("%s", log)
		}
	}

	return stdout, nil
}
