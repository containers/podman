package firewall

// Copyright 2016 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import (
	"net"

	"github.com/containernetworking/cni/pkg/types/current"
)

// FirewallNetConf represents the firewall configuration.
type FirewallNetConf struct {
	//types.NetConf

	// IptablesAdminChainName is an optional name to use instead of the default
	// admin rules override chain name that includes the interface name.
	IptablesAdminChainName string

	// FirewalldZone is an optional firewalld zone to place the interface into.  If
	// the firewalld backend is used but the zone is not given, it defaults
	// to 'trusted'
	FirewalldZone string

	PrevResult    *current.Result
}

// FirewallBackend is an interface to the system firewall, allowing addition and
// removal of firewall rules.
type FirewallBackend interface {
	Add(*FirewallNetConf) error
	Del(*FirewallNetConf) error
}

func ipString(ip net.IPNet) string {
	if ip.IP.To4() == nil {
		return ip.IP.String() + "/128"
	}
	return ip.IP.String() + "/32"
}
