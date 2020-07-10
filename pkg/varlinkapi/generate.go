// +build varlink

package varlinkapi

import (
	"encoding/json"

	iopodman "github.com/containers/podman/v2/pkg/varlink"
)

// GenerateKube ...
func (i *VarlinkAPI) GenerateKube(call iopodman.VarlinkCall, name string, service bool) error {
	pod, serv, err := GenerateKube(name, service, i.Runtime)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	podB, err := json.Marshal(pod)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	servB, err := json.Marshal(serv)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	return call.ReplyGenerateKube(iopodman.KubePodService{
		Pod:     string(podB),
		Service: string(servB),
	})
}
