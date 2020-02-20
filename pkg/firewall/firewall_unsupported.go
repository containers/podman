// +build !linux

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
func GetBackend(backend string) (FirewallBackend, error) {
	return nil, fmt.Errorf("firewall backends are not presently supported on this OS")
}
