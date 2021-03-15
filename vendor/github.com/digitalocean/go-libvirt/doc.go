// Copyright 2016 The go-libvirt Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*
	Package libvirt provides a pure Go interface for Libvirt.

	Rather than using Libvirt's C bindings, this package makes use of
	Libvirt's RPC interface, as documented here: https://libvirt.org/internals/rpc.html.
	Connections to the libvirt server may be local, or remote. RPC packets are encoded
	using the XDR standard as defined by RFC 4506.

	This should be considered a work in progress. Most functionaly provided by the C
	bindings have not yet made their way into this library. Pull requests are welcome!
	The definition of the RPC protocol is in the libvirt source tree under src/rpc/virnetprotocol.x.

	Example usage:

	package main

	import (
		"fmt"
		"log"
		"net"
		"time"

		"github.com/digitalocean/go-libvirt"
	)

	func main() {
		//c, err := net.DialTimeout("tcp", "127.0.0.1:16509", 2*time.Second)
		//c, err := net.DialTimeout("tcp", "192.168.1.12:16509", 2*time.Second)
		c, err := net.DialTimeout("unix", "/var/run/libvirt/libvirt-sock", 2*time.Second)
		if err != nil {
			log.Fatalf("failed to dial libvirt: %v", err)
		}

		l := libvirt.New(c)
		if err := l.Connect(); err != nil {
			log.Fatalf("failed to connect: %v", err)
		}

		v, err := l.Version()
		if err != nil {
			log.Fatalf("failed to retrieve libvirt version: %v", err)
		}
		fmt.Println("Version:", v)

		domains, err := l.Domains()
		if err != nil {
			log.Fatalf("failed to retrieve domains: %v", err)
		}

		fmt.Println("ID\tName\t\tUUID")
		fmt.Printf("--------------------------------------------------------\n")
		for _, d := range domains {
			fmt.Printf("%d\t%s\t%x\n", d.ID, d.Name, d.UUID)
		}

		if err := l.Disconnect(); err != nil {
			log.Fatal("failed to disconnect: %v", err)
		}
	}
*/

package libvirt
