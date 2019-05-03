// +build linux

package libpod

import (
	"strconv"
	"strings"

	"github.com/containers/psgo"
	"github.com/pkg/errors"
)

// Top gathers statistics about the running processes in a container. It returns a
// []string for output
func (c *Container) Top(descriptors []string) ([]string, error) {
	conStat, err := c.State()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to look up state for %s", c.ID())
	}
	if conStat != ContainerStateRunning {
		return nil, errors.Errorf("top can only be used on running containers")
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
	return c.GetContainerPidInformation(psgoDescriptors)
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
	// TODO: psgo returns a [][]string to give users the ability to apply
	//       filters on the data.  We need to change the API here and the
	//       varlink API to return a [][]string if we want to make use of
	//       filtering.
	psgoOutput, err := psgo.JoinNamespaceAndProcessInfo(pid, descriptors)
	if err != nil {
		return nil, err
	}
	res := []string{}
	for _, out := range psgoOutput {
		res = append(res, strings.Join(out, "\t"))
	}
	return res, nil
}

// GetContainerPidInformationDescriptors returns a string slice of all supported
// format descriptors of GetContainerPidInformation.
func GetContainerPidInformationDescriptors() ([]string, error) {
	return psgo.ListDescriptors(), nil
}
