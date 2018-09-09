// +build linux

// Copyright 2018 CNI authors
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
	"strings"

	"github.com/godbus/dbus"
)

const (
	dbusName               = "org.freedesktop.DBus"
	dbusPath               = "/org/freedesktop/DBus"
	dbusGetNameOwnerMethod = "GetNameOwner"

	firewalldName               = "org.fedoraproject.FirewallD1"
	firewalldPath               = "/org/fedoraproject/FirewallD1"
	firewalldZoneInterface      = "org.fedoraproject.FirewallD1.zone"
	firewalldAddSourceMethod    = "addSource"
	firewalldRemoveSourceMethod = "removeSource"

	errZoneAlreadySet = "ZONE_ALREADY_SET"
)

// Only used for testcases to override the D-Bus connection
var testConn *dbus.Conn

type fwdBackend struct {
	conn *dbus.Conn
}

// fwdBackend implements the FirewallBackend interface
var _ FirewallBackend = &fwdBackend{}

func getConn() (*dbus.Conn, error) {
	if testConn != nil {
		return testConn, nil
	}
	return dbus.SystemBus()
}

// isFirewalldRunning checks whether firewalld is running.
func isFirewalldRunning() bool {
	conn, err := getConn()
	if err != nil {
		return false
	}

	dbusObj := conn.Object(dbusName, dbusPath)
	var res string
	if err := dbusObj.Call(dbusName+"."+dbusGetNameOwnerMethod, 0, firewalldName).Store(&res); err != nil {
		return false
	}

	return true
}

func newFirewalldBackend() (FirewallBackend, error) {
	conn, err := getConn()
	if err != nil {
		return nil, err
	}

	backend := &fwdBackend{
		conn: conn,
	}
	return backend, nil
}

func getFirewalldZone(conf *FirewallNetConf) string {
	if conf.FirewalldZone != "" {
		return conf.FirewalldZone
	}

	return "trusted"
}

func (fb *fwdBackend) Add(conf *FirewallNetConf) error {
	zone := getFirewalldZone(conf)

	for _, ip := range conf.PrevResult.IPs {
		ipStr := ipString(ip.Address)
		// Add a firewalld rule which assigns the given source IP to the given zone
		firewalldObj := fb.conn.Object(firewalldName, firewalldPath)
		var res string
		if err := firewalldObj.Call(firewalldZoneInterface+"."+firewalldAddSourceMethod, 0, zone, ipStr).Store(&res); err != nil {
			if !strings.Contains(err.Error(), errZoneAlreadySet) {
				return fmt.Errorf("failed to add the address %v to %v zone: %v", ipStr, zone, err)
			}
		}
	}
	return nil
}

func (fb *fwdBackend) Del(conf *FirewallNetConf) error {
	for _, ip := range conf.PrevResult.IPs {
		ipStr := ipString(ip.Address)
		// Remove firewalld rules which assigned the given source IP to the given zone
		firewalldObj := fb.conn.Object(firewalldName, firewalldPath)
		var res string
		firewalldObj.Call(firewalldZoneInterface+"."+firewalldRemoveSourceMethod, 0, getFirewalldZone(conf), ipStr).Store(&res)
	}
	return nil
}
