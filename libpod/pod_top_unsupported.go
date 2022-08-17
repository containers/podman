//go:build !linux
// +build !linux

package libpod

import (
	"errors"
)

// GetPodPidInformation returns process-related data of all processes in
// the pod.  The output data can be controlled via the `descriptors`
// argument which expects format descriptors and supports all AIXformat
// descriptors of ps (1) plus some additional ones to for instance inspect the
// set of effective capabilities.  Each element in the returned string slice
// is a tab-separated string.
//
// For more details, please refer to github.com/containers/psgo.
func (p *Pod) GetPodPidInformation(descriptors []string) ([]string, error) {
	return nil, errors.New("not implemented (*Pod) GetPodPidInformation")
}
