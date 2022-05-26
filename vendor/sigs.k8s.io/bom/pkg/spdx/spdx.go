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
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"sigs.k8s.io/release-utils/util"
)

const (
	defaultDocumentAuthor   = "Kubernetes Release Managers (release-managers@kubernetes.io)"
	archiveManifestFilename = "manifest.json"
	spdxTempDir             = "spdx"
	spdxLicenseData         = spdxTempDir + "/licenses"
	spdxLicenseDlCache      = spdxTempDir + "/downloadCache"
	gitIgnoreFile           = ".gitignore"

	// Consts of some SPDX expressions
	NONE        = "NONE"
	NOASSERTION = "NOASSERTION"

	termBanner = `ICAgICAgICAgICAgICAgXyAgICAgIAogX19fIF8gX18gICBfX3wgfF8gIF9fCi8gX198ICdfIFwg
LyBfYCBcIFwvIC8KXF9fIFwgfF8pIHwgKF98IHw+ICA8IAp8X19fLyAuX18vIFxfXyxfL18vXF9c
CiAgICB8X3wgICAgICAgICAgICAgICAK`
)

// https://spdx.github.io/spdx-spec/3-package-information/#32-package-spdx-identifier
var validIDCharsRe = regexp.MustCompile(`[^a-zA-Z0-9-.]+`)

type SPDX struct {
	impl    spdxImplementation
	options *Options
}

func NewSPDX() *SPDX {
	return &SPDX{
		impl:    &spdxDefaultImplementation{},
		options: &defaultSPDXOptions,
	}
}

func (spdx *SPDX) SetImplementation(impl spdxImplementation) {
	spdx.impl = impl
}

type Options struct {
	AnalyzeLayers    bool
	NoGitignore      bool     // Do not read exclusions from gitignore file
	ProcessGoModules bool     // If true, spdx will check if dirs are go modules and analize the packages
	OnlyDirectDeps   bool     // Only include direct dependencies from go.mod
	ScanLicenses     bool     // Scan licenses from everypossible place unless false
	AddTarFiles      bool     // Scan and add files inside of tarfiles
	LicenseCacheDir  string   // Directory to cache SPDX license downloads
	LicenseData      string   // Directory to store the SPDX licenses
	IgnorePatterns   []string // Patterns to ignore when scanning file
}

func (spdx *SPDX) Options() *Options {
	return spdx.options
}

var defaultSPDXOptions = Options{
	LicenseCacheDir:  filepath.Join(os.TempDir(), spdxLicenseDlCache),
	LicenseData:      filepath.Join(os.TempDir(), spdxLicenseData),
	AnalyzeLayers:    true,
	ProcessGoModules: true,
	IgnorePatterns:   []string{},
	ScanLicenses:     true,
}

type ArchiveManifest struct {
	ConfigFilename string   `json:"Config"`
	RepoTags       []string `json:"RepoTags"`
	LayerFiles     []string `json:"Layers"`
}

// ImageOptions set of options for processing tar files
type TarballOptions struct {
	ExtractDir string // Directory where the docker tar archive will be extracted
	AddFiles   bool
}

// buildIDString takes a list of seed strings and builds a
// valid SPDX ID string from them. If none is supplied, an
// ID using an UUID will be returned
func buildIDString(seeds ...string) string {
	validSeeds := []string{}
	numValidSeeds := 0
	for _, s := range seeds {
		// Replace some chars with - to keep the sense of the ID
		for _, r := range []string{"/", ":"} {
			s = strings.ReplaceAll(s, r, "-")
		}
		// Replace invalid chars with unicode numbers to avoid collisions
		s = validIDCharsRe.ReplaceAllStringFunc(s, func(s string) string {
			r := ""
			for i := 0; i < len(s); i++ {
				uc, _ := utf8.DecodeRuneInString(string(s[i]))
				r = fmt.Sprintf("%sC%d", r, uc)
			}
			return r
		})
		if s != "" {
			validSeeds = append(validSeeds, s)
			if !strings.HasPrefix(s, "SPDXRef-") {
				numValidSeeds++
			}
		}
	}

	// If we did not get any seeds, use an UUID
	if numValidSeeds == 0 {
		validSeeds = append(validSeeds, uuid.New().String())
	}

	id := ""
	for _, s := range validSeeds {
		if id != "" {
			id += "-"
		}
		id += s
	}
	return id
}

// PackageFromDirectory indexes all files in a directory and builds a
// SPDX package describing its contents
func (spdx *SPDX) PackageFromDirectory(dirPath string) (pkg *Package, err error) {
	pkg, err = spdx.impl.PackageFromDirectory(spdx.options, dirPath)
	if err != nil {
		return nil, errors.Wrap(err, "generating SPDX package from directory")
	}

	// Scan the directory contents and if it is a go module, process the
	// dependencies
	if util.Exists(filepath.Join(dirPath, GoModFileName)) && spdx.Options().ProcessGoModules {
		logrus.Info("Directory contains a go module. Scanning go packages")
		deps, err := spdx.impl.GetGoDependencies(dirPath, spdx.Options())
		if err != nil {
			return nil, errors.Wrap(err, "scanning go packages")
		}
		logrus.Infof("Go module built list of %d dependencies", len(deps))
		for _, dep := range deps {
			if err := pkg.AddDependency(dep); err != nil {
				return nil, errors.Wrap(err, "adding go dependency")
			}
		}
	}

	return pkg, nil
}

// PackageFromImageTarball returns a SPDX package from a tarball
func (spdx *SPDX) PackageFromImageTarball(tarPath string) (imagePackage *Package, err error) {
	return spdx.impl.PackageFromImageTarball(spdx.Options(), tarPath)
}

// PackageFromArchive returns a SPDX package from a tarball
func (spdx *SPDX) PackageFromArchive(archivePath string) (imagePackage *Package, err error) {
	if strings.HasSuffix(archivePath, "tar") || strings.HasSuffix(archivePath, "tar.gz") {
		return spdx.impl.PackageFromTarball(
			spdx.Options(), &TarballOptions{
				AddFiles: true,
			}, archivePath,
		)
	}
	return nil, errors.Wrap(err, "unable to create spdx package from archive, only tar archives are supported")
}

// FileFromPath creates a File object from a path
func (spdx *SPDX) FileFromPath(filePath string) (*File, error) {
	if !util.Exists(filePath) {
		return nil, errors.New("file does not exist")
	}
	f := NewFile()
	if err := f.ReadSourceFile(filePath); err != nil {
		return nil, errors.Wrap(err, "creating file from path")
	}
	return f, nil
}

// AnalyzeLayer uses the collection of image analyzers to see if
//  it matches a known image from which a spdx package can be
//  enriched with more information
func (spdx *SPDX) AnalyzeImageLayer(layerPath string, pkg *Package) error {
	return spdx.impl.AnalyzeImageLayer(layerPath, pkg)
}

// ExtractTarballTmp extracts a tarball to a temp file
func (spdx *SPDX) ExtractTarballTmp(tarPath string) (tmpDir string, err error) {
	return spdx.impl.ExtractTarballTmp(tarPath)
}

// PullImagesToArchive
func (spdx *SPDX) PullImagesToArchive(reference, path string) ([]struct {
	Reference string
	Archive   string
	Arch      string
	OS        string
}, error) {
	return spdx.impl.PullImagesToArchive(reference, path)
}

// ImageRefToPackage gets an image reference (tag or digest) and returns
// a spdx package describing it. It can take two forms:
//  - When the reference is a digest (or single image), a single package
//    describing the layers is returned
//  - When the reference is an image index, the returned package is a
//    package referencing each of the images, each in its own packages.
//  All subpackages are returned with a relationship of VARIANT_OF
func (spdx *SPDX) ImageRefToPackage(reference string) (pkg *Package, err error) {
	return spdx.impl.ImageRefToPackage(reference, spdx.Options())
}

func Banner() string {
	d, err := base64.StdEncoding.DecodeString(termBanner)
	if err != nil {
		return ""
	}
	return string(d)
}
