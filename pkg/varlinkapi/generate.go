// +build varlink

package varlinkapi

import (
	"encoding/json"
	"github.com/containers/libpod/cmd/podman/shared"
	iopodman "github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/pkg/systemdgen"
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

// GenerateSystemd ...
func (i *LibpodAPI) GenerateSystemd(call iopodman.VarlinkCall, nameOrID, restart string, stopTimeout int64, useName bool) error {
	ctr, err := i.Runtime.LookupContainer(nameOrID)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	timeout := int(ctr.StopTimeout())
	if stopTimeout >= 0 {
		timeout = int(stopTimeout)
	}
	name := ctr.ID()
	if useName {
		name = ctr.Name()
	}
	unit, err := systemdgen.CreateSystemdUnitAsString(name, ctr.ID(), restart, ctr.Config().StaticDir, timeout)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyGenerateSystemd(unit)
}
