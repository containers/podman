// +build varlink

package varlinkapi

import (
	"encoding/json"
	"github.com/containers/libpod/cmd/podman/shared"
	iopodman "github.com/containers/libpod/cmd/podman/varlink"
)

// GenerateKube ...
func (i *LibpodAPI) GenerateKube(call iopodman.VarlinkCall, name string, service bool) error {
	pod, serv, err := shared.GenerateKube(name, service, i.Runtime)
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
