package cgroups

import (
	"fmt"
	"path/filepath"
	"strings"

	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/godbus/dbus/v5"
)

func systemdCreate(path string, c *systemdDbus.Conn) error {
	slice, name := filepath.Split(path)
	slice = strings.TrimSuffix(slice, "/")

	var lastError error
	for i := 0; i < 2; i++ {
		properties := []systemdDbus.Property{
			systemdDbus.PropDescription(fmt.Sprintf("cgroup %s", name)),
			systemdDbus.PropWants(slice),
		}
		pMap := map[string]bool{
			"DefaultDependencies": false,
			"MemoryAccounting":    true,
			"CPUAccounting":       true,
			"BlockIOAccounting":   true,
		}
		if i == 0 {
			pMap["Delegate"] = true
		}
		for k, v := range pMap {
			p := systemdDbus.Property{
				Name:  k,
				Value: dbus.MakeVariant(v),
			}
			properties = append(properties, p)
		}

		ch := make(chan string)
		_, err := c.StartTransientUnit(name, "replace", properties, ch)
		if err != nil {
			lastError = err
			continue
		}
		<-ch
		return nil
	}
	return lastError
}

/*
   systemdDestroyConn is copied from containerd/cgroups/systemd.go file, that
   has the following license:

   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/
func systemdDestroyConn(path string, c *systemdDbus.Conn) error {
	name := filepath.Base(path)

	ch := make(chan string)
	_, err := c.StopUnit(name, "replace", ch)
	if err != nil {
		return err
	}
	<-ch
	return nil
}
