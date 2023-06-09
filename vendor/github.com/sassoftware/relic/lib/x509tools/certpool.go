//
// Copyright (c) SAS Institute Inc.
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
//

package x509tools

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
)

// Load a certificate pool from a file and set it as the root CA for a TLS
// config. If path is empty then the system pool will be used. If the filename
// starts with + then both the system pool and the contents of the file will be
// used.
func LoadCertPool(path string, tconf *tls.Config) error {
	if path == "" {
		return nil
	} else if path[0] == '+' {
		pool, err := x509.SystemCertPool()
		if err != nil {
			return err
		}
		tconf.RootCAs = pool
		path = path[1:]
	} else {
		tconf.RootCAs = x509.NewCertPool()
	}
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	if !tconf.RootCAs.AppendCertsFromPEM(contents) {
		return fmt.Errorf("no CA certificates in %s", path)
	}
	return nil
}
