/*
Copyright 2021 The Kubernetes Authors.

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

package cve

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"k8s.io/release/pkg/release"
	"sigs.k8s.io/release-sdk/object"
)

const (
	Bucket       = release.TestBucket
	Directory    = "/release/cve/"
	mapExt       = ".yaml"
	newMapHeader = `---
# This is a new CVE entry data map. Complete all required sections, save
# and exit to publish. If you need to cancel don't save the file or delete
# everything, save and exit.
`
	// Regexp to check CVE IDs
	CVEIDRegExp = `^CVE-\d{4}-\d+$`
)

type Client struct {
	impl    ClientImplementation
	options ClientOptions
}

type ClientOptions struct {
	Bucket    string
	Directory string
}

var cveDefaultOpts = ClientOptions{
	Bucket:    Bucket,
	Directory: Directory,
}

func NewClient() *Client {
	return &Client{
		impl:    &defaultClientImplementation{},
		options: cveDefaultOpts,
	}
}

// Write writes a map to the bucket
func (c *Client) Write(cve, mapPath string) error {
	if err := c.impl.CheckID(cve); err != nil {
		return errors.Wrap(err, "checking CVE identifier")
	}

	// Validate the information in the data maps
	if err := c.impl.ValidateCVEMap(cve, mapPath, &c.options); err != nil {
		return errors.Wrap(err, "validating CVE data in map file")
	}

	destPath := object.GcsPrefix + filepath.Join(
		c.options.Bucket, c.options.Directory, cve+".yaml",
	)

	// Copy the map into the bucket
	if err := c.impl.CopyFile(
		mapPath, destPath, &c.options,
	); err != nil {
		return errors.Wrapf(err, "writing %s map file to CVE bucket", cve)
	}

	return nil
}

// CheckID checks a CVE ID to verify it is well formed
func (c *Client) CheckID(cve string) error {
	return c.impl.CheckID(cve)
}

// Delete removes a CVE entry from the security bucket location
func (c *Client) Delete(cve string) error {
	if err := c.impl.CheckID(cve); err != nil {
		return errors.Wrap(err, "checking CVE identifier")
	}

	return c.impl.DeleteFile(
		object.GcsPrefix+filepath.Join(
			c.options.Bucket, c.options.Directory, cve+".yaml",
		), &c.options,
	)
}

// CopyToTemp copies a CVE entry into a temporary local file
func (c *Client) CopyToTemp(cve string) (file *os.File, err error) {
	return c.impl.CopyToTemp(cve, &c.options)
}

// CreateEmptyMap creates a new, empty CVE data map
func (c *Client) CreateEmptyMap(cve string) (file *os.File, err error) {
	return c.impl.CreateEmptyFile(cve, &c.options)
}

// List return a list iof existing CVE entries
func (c *Client) EntryExists(cveID string) (bool, error) {
	return c.impl.EntryExists(cveID, &c.options)
}
