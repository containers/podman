// +build linux

package libpod

import (
	"strconv"
	"strings"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/psgo"
)

// GetPodPidInformation returns process-related data of all processes in
// the pod.  The output data can be controlled via the `descriptors`
// argument which expects format descriptors and supports all AIXformat
// descriptors of ps (1) plus some additional ones to for instance inspect the
// set of effective capabilities.  Eeach element in the returned string slice
// is a tab-separated string.
//
// For more details, please refer to github.com/containers/psgo.
func (p *Pod) GetPodPidInformation(descriptors []string) ([]string, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	pids := make([]string, 0)
	ctrsInPod, err := p.allContainers()
	if err != nil {
		return nil, err
	}
	for _, c := range ctrsInPod {
		c.lock.Lock()

		if err := c.syncContainer(); err != nil {
			c.lock.Unlock()
			return nil, err
		}
		if c.state.State == define.ContainerStateRunning {
			pid := strconv.Itoa(c.state.PID)
			pids = append(pids, pid)
		}
		c.lock.Unlock()
	}
	// TODO: psgo returns a [][]string to give users the ability to apply
	//       filters on the data.  We need to change the API here and the
	//       varlink API to return a [][]string if we want to make use of
	//       filtering.
	opts := psgo.JoinNamespaceOpts{FillMappings: rootless.IsRootless()}
	output, err := psgo.JoinNamespaceAndProcessInfoByPidsWithOptions(pids, descriptors, &opts)
	if err != nil {
		return nil, err
	}
	res := []string{}
	for _, out := range output {
		res = append(res, strings.Join(out, "\t"))
	}
	return res, nil
}
