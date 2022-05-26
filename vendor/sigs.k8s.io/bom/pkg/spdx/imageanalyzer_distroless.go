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

package spdx

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"sigs.k8s.io/bom/pkg/license"
	"sigs.k8s.io/release-utils/http"
	"sigs.k8s.io/release-utils/util"
)

const (
	distrolessBundleURL        = "https://raw.githubusercontent.com/GoogleContainerTools/distroless/master/"
	distrolessBundle           = "package_bundle_amd64_debian10.versions" // TODO: Perhaps make an option
	distrolessLicensePath      = "./usr/share/doc/"
	distrolessLicenseName      = "/copyright"
	distrolessCommonLicenseDir = "/usr/share/common-licenses/"
	commonLicensesRe           = `(?i)/usr/share/common-licenses/[-A-Z0-9\.]+`
	gzExt                      = ".gz"
)

type distrolessHandler struct {
	reader  *license.Reader
	Options *ContainerLayerAnalyzerOptions
}

// ReadPackageData reads the distroless
func (h *distrolessHandler) ReadPackageData(layerPath string, pkg *Package) error {
	// Create a new license reader to scan license files
	licenseReader, err := h.licenseReader(h.Options)
	if err != nil {
		return errors.Wrap(
			err, "creating license reader to scan distroless image",
		)
	}

	// Create the package representing the distroless layer
	pkg.Name = "distroless"
	pkg.ID = "SPDXRef-Package-distroless"
	pkg.FilesAnalyzed = false

	// Fetch the current distrolless package list
	packageList, err := h.fetchDistrolessPackages()
	if err != nil {
		return errors.Wrap(err, "getting package lists")
	}

	// Open the distroless layer tar for reading
	tarfile, err := os.Open(layerPath)
	if err != nil {
		return errors.Wrap(err, "opening distroless image layer ")
	}
	defer tarfile.Close()
	dir, err := os.MkdirTemp(os.TempDir(), "image-process-")
	if err != nil {
		return errors.Wrap(err, "creating temporary directory")
	}
	defer os.RemoveAll(dir)
	var tr *tar.Reader
	if filepath.Ext(layerPath) == gzExt {
		gzf, err := gzip.NewReader(tarfile)
		if err != nil {
			return errors.Wrap(err, "creating gzip reader")
		}
		tr = tar.NewReader(gzf)
	} else {
		tr = tar.NewReader(tarfile)
	}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrap(err, "reading the image tarfile")
		}

		// Scan the license directories to to determine the installed packages
		if strings.HasPrefix(hdr.Name, distrolessLicensePath) && strings.HasSuffix(hdr.Name, distrolessLicenseName) {
			// We infer the name of the package from the license directory
			packageName := strings.TrimSuffix(strings.TrimPrefix(hdr.Name, distrolessLicensePath), distrolessLicenseName)
			logrus.Infof("Creating SPDX subpackage " + packageName)
			subpkg := NewPackage()
			subpkg.Name = packageName
			if _, ok := packageList[subpkg.Name]; ok {
				logrus.Infof(" distroless uses version %s of %s", packageList[subpkg.Name], subpkg.Name)
				subpkg.Version = packageList[subpkg.Name]
			} else {
				logrus.Warnf("could not determine version for package %s", subpkg.Name)
			}

			// Extract the package license to a file
			f, err := os.Create(filepath.Join(dir, packageName+".license"))
			if err != nil {
				return errors.Wrap(err, "creating image layer file")
			}
			defer f.Close()

			if _, err := io.CopyN(f, tr, 1024); err != nil {
				if err == io.EOF {
					break
				}

				return errors.Wrap(err, "extracting license data for "+subpkg.Name)
			}

			// Use our license classifier to try to determine
			// the license we are dealing with
			spdxlicense, err := licenseReader.LicenseFromFile(f.Name())
			if err != nil {
				return errors.Wrap(err, "reading license from file")
			}

			// If we still do not have a license, try to get it from the
			// devian copyright files. We have to read the files so...
			if spdxlicense == nil {
				// ...open the file
				fileData, err := ioutil.ReadFile(filepath.Join(dir, packageName+".license"))
				if err != nil {
					return errors.Wrap(err, "reading license file")
				}

				// We will try to look for the license in two ways:
				if strings.Contains(string(fileData), "is in the public domain") {
					// Option 1: File is in the public domain
					logrus.Info("File is the public domain")

					// In this case we include the full license text in the manifest
					subpkg.CopyrightText = string(fileData)
					subpkg.LicenseComments = "Found public domain declaration in copyright text file"
				} else {
					// Option 2: Copyright file references an installed license.
					re := regexp.MustCompile(commonLicensesRe)
					label := re.FindString(string(fileData))
					label = strings.TrimPrefix(label, distrolessCommonLicenseDir)
					label = strings.TrimSuffix(label, ".")

					// Translate from debian to SPDX label
					label = license.DebianLicenseLabels[label]
					if label != "" {
						spdxlicense = licenseReader.LicenseFromLabel(label)
						logrus.Infof("Found license %s for package %s by reading copyright file", spdxlicense.LicenseID, subpkg.Name)
						subpkg.LicenseDeclared = spdxlicense.LicenseID
					}
				}
			} else {
				subpkg.LicenseDeclared = spdxlicense.LicenseID
			}

			// Add the debian package to the layer package
			if err := pkg.AddPackage(subpkg); err != nil {
				return errors.Wrapf(err, "adding %s subpackage", subpkg.Name)
			}
		}
	}
	return nil
}

// fetchDistrolessPackages retrieves the package list published at the
//  distroless repository keyed by package name and version
func (h *distrolessHandler) fetchDistrolessPackages() (pkgInfo map[string]string, err error) {
	logrus.Info("Fetching distroless image package list")
	body, err := http.NewAgent().Get(distrolessBundleURL + distrolessBundle)
	if err != nil {
		return nil, errors.Wrap(err, "fetching distroless image package manifest")
	}

	pkgInfo = map[string]string{}
	if err := json.Unmarshal(body, &pkgInfo); err != nil {
		return nil, errors.Wrap(err, "unmarshalling the distroless package list")
	}
	logrus.Infof(
		"Distroless bundle for %s lists %d packages",
		distrolessBundle, len(pkgInfo),
	)
	return pkgInfo, nil
}

// licenseReader returns a reusable license reader
func (h *distrolessHandler) licenseReader(o *ContainerLayerAnalyzerOptions) (*license.Reader, error) {
	if h.reader == nil {
		logrus.Info("Initializing licence reader with default options")
		// We use a default license cache
		opts := license.DefaultReaderOptions
		ldir := filepath.Join(os.TempDir(), "spdx-license-reader-licenses")
		// ... unless overridden by the options
		if o.LicenseCacheDir != "" {
			ldir = o.LicenseCacheDir
		}

		// If the license cache does not exist, create it
		if !util.Exists(ldir) {
			if err := os.MkdirAll(ldir, os.FileMode(0o0755)); err != nil {
				return nil, errors.Wrap(err, "creating license cache directory")
			}
		}
		opts.CacheDir = ldir
		// Create the new reader
		reader, err := license.NewReaderWithOptions(opts)
		if err != nil {
			return nil, errors.Wrap(err, "creating reusable license reader")
		}
		h.reader = reader
	}
	return h.reader, nil
}

// CanHandle returns a bools indicating if this handle can supply more
// data about the specified tarball
func (h *distrolessHandler) CanHandle(layerPath string) (can bool, err error) {
	// Open the tar file
	f, err := os.Open(layerPath)
	if err != nil {
		return can, errors.Wrap(err, "opening tarball")
	}

	var tr *tar.Reader
	if filepath.Ext(layerPath) == gzExt {
		gzf, err := gzip.NewReader(f)
		if err != nil {
			return can, errors.Wrap(err, "creating gzip reader")
		}
		tr = tar.NewReader(gzf)
	} else {
		tr = tar.NewReader(f)
	}
	b := bytes.NewBuffer(make([]byte, 0))
	// Search for the os-file in the tar contents
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return can, errors.Wrap(err, "reading the image tarfile")
		}

		if hdr.FileInfo().IsDir() {
			continue
		}

		// Scan for the os-release file in the tarball
		if hdr.Name == "./etc/os-release" {
			if _, err = io.CopyN(b, tr, 1024); err != nil {
				if err == io.EOF {
					break
				}

				return can, errors.Wrap(err, "extracting os-release file")
			}
		}
	}
	// If the image has the Distroless tag in the OS file, we can handle it
	if strings.Contains(b.String(), `PRETTY_NAME="Distroless"`) {
		logrus.Infof("👍 Tarball %s identified as distroless layer", layerPath)
		return true, nil
	}
	return can, nil
}
