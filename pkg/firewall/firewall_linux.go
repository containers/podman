// +build linux

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

package firewall

import (
	"fmt"
)

// GetBackend retrieves a firewall backend for adding or removing firewall rules
// on the system.
// Valid backend names are firewalld, iptables, and none.
// If the empty string is given, a firewalld backend will be returned if
// firewalld is running, and an iptables backend will be returned otherwise.
func GetBackend(backend string) (FirewallBackend, error) {
	switch backend {
	case "firewalld":
		return newFirewalldBackend()
	case "iptables":
		return newIptablesBackend()
	case "none":
		return newNoneBackend()
	case "":
		// Default to firewalld if it's running
		if isFirewalldRunning() {
			return newFirewalldBackend()
		}

		// Otherwise iptables
		return newIptablesBackend()
	default:
		return nil, fmt.Errorf("unrecognized firewall backend %q", backend)
	}
}
