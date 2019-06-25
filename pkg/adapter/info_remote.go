// +build remoteclient

package adapter

import (
	"encoding/json"
	"github.com/containers/libpod/libpod/define"

	"github.com/containers/libpod/cmd/podman/varlink"
)

// Info returns information for the host system and its components
func (r RemoteRuntime) Info() ([]define.InfoData, error) {
	// TODO the varlink implementation for info should be updated to match the output for regular info
	var (
		reply    []define.InfoData
		hostInfo map[string]interface{}
		store    map[string]interface{}
	)

	registries := make(map[string]interface{})
	insecureRegistries := make(map[string]interface{})
	info, err := iopodman.GetInfo().Call(r.Conn)
	if err != nil {
		return nil, err
	}

	// info.host -> map[string]interface{}
	h, err := json.Marshal(info.Host)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(h, &hostInfo)

	// info.store -> map[string]interface{}
	s, err := json.Marshal(info.Store)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(s, &store)

	registries["registries"] = info.Registries
	insecureRegistries["registries"] = info.Insecure_registries

	// Add everything to the reply
	reply = append(reply, define.InfoData{Type: "host", Data: hostInfo})
	reply = append(reply, define.InfoData{Type: "registries", Data: registries})
	reply = append(reply, define.InfoData{Type: "insecure registries", Data: insecureRegistries})
	reply = append(reply, define.InfoData{Type: "store", Data: store})
	return reply, nil
}
