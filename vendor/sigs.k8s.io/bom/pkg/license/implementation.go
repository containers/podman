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
	"regexp"

	licenseclassifier "github.com/google/licenseclassifier/v2"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ReaderDefaultImpl the default license reader imlementation, uses
// Google's cicense classifier
type ReaderDefaultImpl struct {
	lc      *licenseclassifier.Classifier
	catalog *Catalog
}

// ClassifyFile takes a file path and returns the most probable license tag
func (d *ReaderDefaultImpl) ClassifyFile(path string) (licenseTag string, moreTags []string, err error) {
	file, err := os.Open(path)
	if err != nil {
		return licenseTag, nil, errors.Wrap(err, "opening file for analysis")
	}
	defer file.Close()

	// Get the classsification
	matches, err := d.Classifier().MatchFrom(file)
	if len(matches) == 0 {
		logrus.Debugf("File does not match a known license: %s", path)
	}
	var highestConf float64
	moreTags = []string{}
	for _, match := range matches {
		if match.Confidence > highestConf {
			highestConf = match.Confidence
			licenseTag = match.Name
			moreTags = append(moreTags, match.Name)
		}
	}
	return licenseTag, []string{}, nil
}

// ClassifyLicenseFiles takes a list of paths and tries to find return all licenses found in it
func (d *ReaderDefaultImpl) ClassifyLicenseFiles(paths []string) (
	licenseList []*ClassifyResult, unrecognizedPaths []string, err error) {
	licenseList = []*ClassifyResult{}
	// Run the files through the clasifier
	for _, f := range paths {
		label, _, err := d.ClassifyFile(f)
		if err != nil {
			return nil, unrecognizedPaths, errors.Wrap(err, "classifying file")
		}
		if label == "" {
			unrecognizedPaths = append(unrecognizedPaths, f)
			continue
		}
		// Get the license corresponding to the ID label
		license := d.catalog.GetLicense(label)
		if license == nil {
			return nil, unrecognizedPaths,
				errors.Errorf("ID does not correspond to a valid license: '%s'", label)
		}
		licenseText, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, errors.Wrap(err, "reading license text")
		}
		// Apend to the return results
		licenseList = append(licenseList, &ClassifyResult{f, string(licenseText), license})
	}
	if len(paths) != len(licenseList) {
		logrus.Infof(
			"License classifier recognized %d/%d (%d%%) of the license files",
			len(licenseList), len(paths), (len(licenseList)/len(paths))*100,
		)
	}
	return licenseList, unrecognizedPaths, nil
}

// LicenseFromLabel return a spdx license from its label
func (d *ReaderDefaultImpl) LicenseFromLabel(label string) (license *License) {
	return d.catalog.GetLicense(label)
}

// LicenseFromFile a file path and returns its license
func (d *ReaderDefaultImpl) LicenseFromFile(path string) (license *License, err error) {
	// Run the files through the clasifier
	label, _, err := d.ClassifyFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "classifying file")
	}

	if label == "" {
		logrus.Debugf("File does not contain a known license: %s", path)
		return nil, nil
	}

	// Get the license corresponding to the ID label
	license = d.catalog.GetLicense(label)
	if license == nil {
		return nil, errors.Errorf("ID does not correspond to a valid license: %s", label)
	}

	return license, nil
}

// FindLicenseFiles will scan a directory and return files that may be licenses
func (d *ReaderDefaultImpl) FindLicenseFiles(path string) ([]string, error) {
	logrus.Infof("Scanning %s for license files", path)
	licenseList := []string{}
	re := regexp.MustCompile(licenseFilanameRe)
	if err := filepath.Walk(path,
		func(path string, finfo os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Directories are ignored
			if finfo.IsDir() {
				return nil
			}

			// No go source files are considered
			if filepath.Ext(path) == ".go" {
				return nil
			}
			// Check if tehe file matches the license regexp
			if re.MatchString(filepath.Base(path)) {
				licenseList = append(licenseList, path)
			}
			return nil
		}); err != nil {
		return nil, errors.Wrap(err, "scanning the directory for license files")
	}
	logrus.Infof("%d license files found in directory", len(licenseList))
	return licenseList, nil
}

// Initialize checks the options and creates the needed objects
func (d *ReaderDefaultImpl) Initialize(opts *ReaderOptions) error {
	// Validate our options before startin
	if err := opts.Validate(); err != nil {
		return errors.Wrap(err, "validating the license reader options")
	}

	// Create the implementation's SPDX object
	catalogOpts := DefaultCatalogOpts
	catalogOpts.CacheDir = opts.CachePath()
	catalog, err := NewCatalogWithOptions(catalogOpts)
	if err != nil {
		return errors.Wrap(err, "creating SPDX object")
	}
	d.catalog = catalog

	if err := d.catalog.LoadLicenses(); err != nil {
		return errors.Wrap(err, "loading licenses")
	}

	logrus.Infof("Writing license data to %s", opts.CachePath())

	// Write the licenses to disk as th classifier will need them
	if err := catalog.WriteLicensesAsText(opts.LicensesPath()); err != nil {
		return errors.Wrap(err, "writing license data to disk")
	}

	// Create the implementation's classifier
	d.lc = licenseclassifier.NewClassifier(opts.ConfidenceThreshold)
	return errors.Wrap(d.lc.LoadLicenses(opts.LicensesPath()), "loading licenses at init")
}

// Classifier returns the license classifier
func (d *ReaderDefaultImpl) Classifier() *licenseclassifier.Classifier {
	return d.lc
}

// SPDX returns the reader's SPDX object
func (d *ReaderDefaultImpl) Catalog() *Catalog {
	return d.catalog
}
