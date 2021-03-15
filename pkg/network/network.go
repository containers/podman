package network

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/containernetworking/cni/libcni"
)

// GetCNIPlugins returns a list of plugins that a given network
// has in the form of a string
func GetCNIPlugins(list *libcni.NetworkConfigList) string {
	plugins := make([]string, 0, len(list.Plugins))
	for _, plug := range list.Plugins {
		plugins = append(plugins, plug.Network.Type)
	}
	return strings.Join(plugins, ",")
}

// GetNetworkID return the network ID for a given name.
// It is just the sha256 hash but this should be good enough.
// The caller has to make sure it is only called with the network name.
func GetNetworkID(name string) string {
	hash := sha256.Sum256([]byte(name))
	return hex.EncodeToString(hash[:])
}
