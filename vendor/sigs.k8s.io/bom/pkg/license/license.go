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

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

package license

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/release-utils/util"
)

const (
	licenseFilanameRe    = `(?i).*license.*`
	defaultCacheSubDir   = "cache"
	defaultLicenseSubDir = "licenses"
)

const kubernetesBoilerPlate = `# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0`

// DebianLicenseLabels is a map to get the SPDX label from a debian label
var DebianLicenseLabels = map[string]string{
	"Apache-2.0": "Apache-2.0",
	"Artistic":   "Artistic-1.0-Perl",
	"BSD":        "BSD-1-Clause",
	"CC0-1.0":    "CC0-1.0",
	"GFDL-1.2":   "GFDL-1.2",
	"GFDL-1.3":   "GFDL-1.3",
	"GPL":        "GPL-1.0",
	"GPL-1":      "GPL-1.0",
	"GPL-2":      "GPL-2.0",
	"GPL-3":      "GPL-3.0",
	"LGPL-2":     "LGPL-2.0",
	"LGPL-2.1":   "LGPL-2.1",
	"LGPL-3":     "LGPL-3.0",
	"MPL-1.1":    "MPL-1.1",
	"MPL-2.0":    "MPL-2.0",
}

// Reader is an object that finds and interprets license files
type Reader struct {
	impl    ReaderImplementation
	Options *ReaderOptions
}

// SetImplementation sets the implementation that the license reader will use
func (r *Reader) SetImplementation(i ReaderImplementation) error {
	r.impl = i
	return errors.Wrap(
		r.impl.Initialize(r.Options),
		"initializing the reader implementation",
	)
}

// NewReader returns a license reader with the default options
func NewReader() (*Reader, error) {
	return NewReaderWithOptions(DefaultReaderOptions)
}

// NewReaderWithOptions returns a new license reader with the specified options
func NewReaderWithOptions(opts *ReaderOptions) (r *Reader, err error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(err, "validating reader options")
	}
	r = &Reader{
		Options: opts,
	}

	if err := r.SetImplementation(&ReaderDefaultImpl{}); err != nil {
		return nil, errors.Wrap(err, "setting the reader implementation")
	}

	return r, nil
}

// ReaderOptions are the optional settings for the license reader
type ReaderOptions struct {
	ConfidenceThreshold float64 // Miniumum confidence to consider a license detected
	WorkDir             string  // Directory where the reader will store its data
	CacheDir            string  // Optional directory where the reader will store its downloads cache
	LicenseDir          string  // Optional dir to store and read the SPDX licenses from
}

// Validate checks the options to verify the are sane
func (ro *ReaderOptions) Validate() error {
	// if there is no working dir, create one
	if ro.WorkDir == "" {
		dir, err := os.MkdirTemp("", "license-reader-")
		if err != nil {
			return errors.Wrap(err, "creating working dir")
		}
		ro.WorkDir = dir
		// Otherwise, check it exists
	} else if _, err := os.Stat(ro.WorkDir); err != nil {
		return errors.Wrap(err, "checking working directory")
	}

	// TODO check dirs
	return nil
}

// CachePath return the full path to the downloads cache
func (ro *ReaderOptions) CachePath() string {
	if ro.CacheDir != "" {
		return ro.CacheDir
	}

	return filepath.Join(ro.WorkDir, defaultCacheSubDir)
}

// LicensesPath return the full path the dir where the licenses are
func (ro *ReaderOptions) LicensesPath() string {
	if ro.LicenseDir != "" {
		return ro.LicenseDir
	}

	return filepath.Join(ro.WorkDir, defaultLicenseSubDir)
}

// DefaultReaderOptions is the default set of options for the classifier
var DefaultReaderOptions = &ReaderOptions{
	ConfidenceThreshold: 0.9,
}

// LicenseFromLabel returns a spdx license from its label
func (r *Reader) LicenseFromLabel(label string) (license *License) {
	return r.impl.LicenseFromLabel(label)
}

// LicenseFromFile reads a file ans returns its license
func (r *Reader) LicenseFromFile(filePath string) (license *License, err error) {
	license, err = r.impl.LicenseFromFile(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "classifying file to determine license")
	}
	return license, err
}

// ReadTopLicense returns the topmost license file in a directory
func (r *Reader) ReadTopLicense(path string) (*ClassifyResult, error) {
	licenseFilePath := ""
	// First, if we have a topmost license, we use that one
	commonNames := []string{"LICENSE", "LICENSE.txt", "COPYING", "COPYRIGHT"}
	for _, f := range commonNames {
		if util.Exists(filepath.Join(path, f)) {
			licenseFilePath = filepath.Join(path, f)
			break
		}
	}

	if licenseFilePath != "" {
		result, _, err := r.impl.ClassifyLicenseFiles([]string{licenseFilePath})
		if err != nil {
			return nil, errors.Wrap(err, "scanning topmost license file")
		}
		if len(result) != 0 {
			logrus.Infof("Concluded license %s from %s", result[0].License.LicenseID, licenseFilePath)
			return result[0], nil
		}
	}

	// If the standard file did not work, then we try to
	// find the license file at the highest dir in the FS tree
	var res *ClassifyResult
	bestPartsN := 0
	licenseFiles, err := r.impl.FindLicenseFiles(path)
	if err != nil {
		return nil, errors.Wrap(err, "finding license files in path")
	}
	for _, fileName := range licenseFiles {
		try := false
		dir := filepath.Dir(fileName)
		parts := strings.Split(dir, string(filepath.Separator))
		// If this file is higher in the fstree, we use it
		if bestPartsN == 0 || len(parts) < bestPartsN {
			try = true
		}
		// If this file in the same level but the path is shorter, we try it
		if len(parts) == bestPartsN && len(fileName) < len(licenseFilePath) {
			try = true
		}

		// If this file is not a better candidate, skip to the next one
		if !try {
			continue
		}

		result, _, err := r.impl.ClassifyLicenseFiles([]string{fileName})
		if err != nil {
			return nil, errors.Wrap(err, "scanning topmost license file")
		}

		// If the file is a license, use it
		if len(result) > 0 {
			bestPartsN = len(parts)
			licenseFilePath = fileName
			res = result[0]
		}
	}
	if res == nil {
		logrus.Warnf("Could not find any licensing information in %s", path)
	} else {
		logrus.Infof("Concluded license %s from %s", res.License.LicenseID, licenseFilePath)
	}
	return res, nil
}

// ReadLicenses returns an array of all licenses found in the specified path
func (r *Reader) ReadLicenses(path string) (
	licenseList []*ClassifyResult, unknownPaths []string, err error,
) {
	licenseFiles, err := r.impl.FindLicenseFiles(path)
	if err != nil {
		return nil, nil, errors.Wrap(err, "searching for license files")
	}

	licenseList, unknownPaths, err = r.impl.ClassifyLicenseFiles(licenseFiles)
	if err != nil {
		return nil, nil, errors.Wrap(err, "classifying found licenses")
	}
	return licenseList, unknownPaths, nil
}

// ClassifyResult abstracts the data resulting from a file classification
type ClassifyResult struct {
	File    string
	Text    string
	License *License
}

//counterfeiter:generate . ReaderImplementation

// ReaderImplementation implements the basic lifecycle of a license reader:
// initializes -> finds license files to scan -> classifies them to a SPDX license
type ReaderImplementation interface {
	Initialize(*ReaderOptions) error
	ClassifyLicenseFiles([]string) ([]*ClassifyResult, []string, error)
	ClassifyFile(string) (string, []string, error)
	LicenseFromFile(string) (*License, error)
	LicenseFromLabel(string) *License
	FindLicenseFiles(string) ([]string, error)
}

// HasKubernetesBoilerPlate checks if a file contains the Kubernetes License boilerplate
func HasKubernetesBoilerPlate(filePath string) (bool, error) {
	// kubernetesBoilerPlate
	sut, err := os.Open(filePath)
	if err != nil {
		return false, errors.Wrap(err, "opening file to check for k8s boilerplate")
	}
	defer sut.Close()

	// Trim whitespace from lines
	scanner := bufio.NewScanner(sut)
	scanner.Split(bufio.ScanLines)
	text := ""
	i := 0
	for scanner.Scan() {
		text = text + scanner.Text() + "\n"
		i++
		if i > 100 {
			break
		}
	}
	// If we're past 100 lines, forget it
	if strings.Contains(text, kubernetesBoilerPlate) {
		logrus.Infof("Found Kubernetes boilerplate in %s", filePath)
		return true, nil
	}

	return false, nil
}

// List abstracts the list of licenses published by SPDX.org
type List struct {
	sync.RWMutex
	Version           string      `json:"licenseListVersion"`
	ReleaseDateString string      `json:"releaseDate "`
	LicenseData       []ListEntry `json:"licenses"`
	Licenses          map[string]*License
}

// Add appends a license to the license list
func (list *List) Add(license *License) {
	list.Lock()
	defer list.Unlock()
	if list.Licenses == nil {
		list.Licenses = map[string]*License{}
	}
	list.Licenses[license.LicenseID] = license
}

// SPDXLicense is a license described in JSON
type License struct {
	IsDeprecatedLicenseID         bool     `json:"isDeprecatedLicenseId"`
	IsFsfLibre                    bool     `json:"isFsfLibre"`
	IsOsiApproved                 bool     `json:"isOsiApproved"`
	LicenseText                   string   `json:"licenseText"`
	StandardLicenseHeaderTemplate string   `json:"standardLicenseHeaderTemplate"`
	StandardLicenseTemplate       string   `json:"standardLicenseTemplate"`
	Name                          string   `json:"name"`
	LicenseID                     string   `json:"licenseId"`
	StandardLicenseHeader         string   `json:"standardLicenseHeader"`
	SeeAlso                       []string `json:"seeAlso"`
}

// WriteText writes the SPDX license text to a text file
func (license *License) WriteText(filePath string) error {
	return errors.Wrap(
		os.WriteFile(
			filePath, []byte(license.LicenseText), os.FileMode(0o644),
		), "while writing license to text file",
	)
}

// ListEntry a license entry in the list
type ListEntry struct {
	IsOsiApproved   bool     `json:"isOsiApproved"`
	IsDeprectaed    bool     `json:"isDeprecatedLicenseId"`
	Reference       string   `json:"reference"`
	DetailsURL      string   `json:"detailsUrl"`
	ReferenceNumber int      `json:"referenceNumber"`
	Name            string   `json:"name"`
	LicenseID       string   `json:"licenseId"`
	SeeAlso         []string `json:"seeAlso"`
}

// ParseLicense parses a SPDX license from its JSON source
func ParseLicense(licenseJSON []byte) (license *License, err error) {
	license = &License{}
	if err := json.Unmarshal(licenseJSON, license); err != nil {
		return nil, errors.Wrap(err, "parsing SPDX licence")
	}
	return license, nil
}
