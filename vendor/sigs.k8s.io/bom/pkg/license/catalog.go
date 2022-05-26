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

package license

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/release-utils/util"
)

// CatalogOptions are the spdx settings
type CatalogOptions struct {
	CacheDir string // Directrory to catch the license we download from SPDX.org
}

// DefaultCatalogOpts are the predetermined settings. License and cache directories
// are in the temporary OS directory and are created if the do not exist
var DefaultCatalogOpts = &CatalogOptions{}

// NewCatalogWithOptions returns a SPDX object with the specified options
func NewCatalogWithOptions(opts *CatalogOptions) (catalog *Catalog, err error) {
	// Create the license downloader
	doptions := DefaultDownloaderOpts
	doptions.CacheDir = opts.CacheDir
	downloader, err := NewDownloaderWithOptions(doptions)
	if err != nil {
		return nil, errors.Wrap(err, "creating downloader")
	}
	catalog = &Catalog{
		Downloader: downloader,
		opts:       opts,
	}

	return catalog, nil
}

// Options returns  a pointer to the catlog options
func (catalog *Catalog) Options() *CatalogOptions {
	return catalog.opts
}

// LoadLicenses reads the license data from the downloader
func (catalog *Catalog) LoadLicenses() error {
	logrus.Info("Loading license data from downloader")
	licenses, err := catalog.Downloader.GetLicenses()
	if err != nil {
		return errors.Wrap(err, "getting licenses from downloader")
	}
	catalog.List = licenses
	logrus.Infof("Got %d licenses from downloader", len(licenses.Licenses))
	return nil
}

// Catalog is an objec to interact with licenses and manifest creation
type Catalog struct {
	Downloader *Downloader     // License Downloader
	List       *List           // List of licenses
	opts       *CatalogOptions // SPDX Options
}

// WriteLicensesAsText writes the SPDX license collection to text files
func (catalog *Catalog) WriteLicensesAsText(targetDir string) error {
	logrus.Infof("Writing %d SPDX licenses to %s", len(catalog.List.Licenses), targetDir)
	if catalog.List.Licenses == nil {
		return errors.New("unable to write licenses, they have not been loaded yet")
	}
	if !util.Exists(targetDir) {
		if err := os.MkdirAll(targetDir, os.FileMode(0o755)); err != nil {
			return errors.Wrap(err, "creating license data dir")
		}
	}
	wg := sync.WaitGroup{}
	var err error
	for _, l := range catalog.List.Licenses {
		wg.Add(1)
		go func(l *License) {
			defer wg.Done()
			if l.IsDeprecatedLicenseID {
				return
			}
			if lerr := l.WriteText(filepath.Join(targetDir, l.LicenseID+".txt")); err != nil {
				if err == nil {
					err = lerr
				} else {
					err = errors.Wrap(err, lerr.Error())
				}
			}
		}(l)
	}
	wg.Wait()
	return errors.Wrap(err, "caught errors while writing license files")
}

// GetLicense returns a license struct from its SPDX ID label
func (catalog *Catalog) GetLicense(label string) *License {
	if lic, ok := catalog.List.Licenses[label]; ok {
		return lic
	}
	logrus.Warn("Label %s is not an identifier of a known license " + label)
	return nil
}
