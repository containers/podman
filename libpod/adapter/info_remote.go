// +build remoteclient

package adapter

import (
	"encoding/json"

	"github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod"
)

// Info returns information for the host system and its components
func (r RemoteRuntime) Info() ([]libpod.InfoData, error) {
	// TODO the varlink implementation for info should be updated to match the output for regular info
	var (
		reply    []libpod.InfoData
		hostInfo map[string]interface{}
		store    map[string]interface{}
	)

	registries := make(map[string]interface{})
	insecureRegistries := make(map[string]interface{})
	conn, err := r.Connect()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	info, err := iopodman.GetInfo().Call(conn)
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
	reply = append(reply, libpod.InfoData{Type: "host", Data: hostInfo})
	reply = append(reply, libpod.InfoData{Type: "registries", Data: registries})
	reply = append(reply, libpod.InfoData{Type: "insecure registries", Data: insecureRegistries})
	reply = append(reply, libpod.InfoData{Type: "store", Data: store})
	return reply, nil
}
