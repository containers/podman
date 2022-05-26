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

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	gitignore "github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/uuid"
	"github.com/nozzle/throttler"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"sigs.k8s.io/bom/pkg/license"
	"sigs.k8s.io/release-utils/util"
)

//counterfeiter:generate . spdxImplementation

type spdxImplementation interface {
	ExtractTarballTmp(string) (string, error)
	ReadArchiveManifest(string) (*ArchiveManifest, error)
	PullImagesToArchive(string, string) ([]struct {
		Reference string
		Archive   string
		Arch      string
		OS        string
	}, error)
	PackageFromImageTarball(*Options, string) (*Package, error)
	PackageFromTarball(*Options, *TarballOptions, string) (*Package, error)
	PackageFromDirectory(*Options, string) (*Package, error)
	GetDirectoryTree(string) ([]string, error)
	IgnorePatterns(string, []string, bool) ([]gitignore.Pattern, error)
	ApplyIgnorePatterns([]string, []gitignore.Pattern) []string
	GetGoDependencies(string, *Options) ([]*Package, error)
	GetDirectoryLicense(*license.Reader, string, *Options) (*license.License, error)
	LicenseReader(*Options) (*license.Reader, error)
	ImageRefToPackage(string, *Options) (*Package, error)
	AnalyzeImageLayer(string, *Package) error
}

type spdxDefaultImplementation struct{}

// ExtractTarballTmp extracts a tarball to a temporary directory
func (di *spdxDefaultImplementation) ExtractTarballTmp(tarPath string) (tmpDir string, err error) {
	tmpDir, err = os.MkdirTemp(os.TempDir(), "spdx-tar-extract-")
	if err != nil {
		return tmpDir, errors.Wrap(err, "creating temporary directory for tar extraction")
	}

	// Open the tar file
	f, err := os.Open(tarPath)
	if err != nil {
		return tmpDir, errors.Wrap(err, "opening tarball")
	}
	defer f.Close()

	var tr *tar.Reader
	if strings.HasSuffix(tarPath, ".gz") || strings.HasSuffix(tarPath, ".tgz") {
		gzipReader, err := gzip.NewReader(f)
		if err != nil {
			return "", err
		}
		tr = tar.NewReader(gzipReader)
	} else {
		tr = tar.NewReader(f)
	}
	numFiles := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return tmpDir, errors.Wrapf(err, "reading tarfile %s", tarPath)
		}

		if hdr.FileInfo().IsDir() {
			continue
		}

		if strings.HasPrefix(filepath.Base(hdr.FileInfo().Name()), ".wh") {
			logrus.Info("Skipping extraction of whithout file")
			continue
		}

		if err := os.MkdirAll(
			filepath.Join(tmpDir, filepath.Dir(hdr.Name)), os.FileMode(0o755),
		); err != nil {
			return tmpDir, errors.Wrap(err, "creating image directory structure")
		}

		targetFile, err := sanitizeExtractPath(tmpDir, hdr.Name)
		if err != nil {
			return tmpDir, err
		}
		f, err := os.Create(targetFile)
		if err != nil {
			return tmpDir, errors.Wrap(err, "creating image layer file")
		}

		if _, err := io.CopyN(f, tr, hdr.Size); err != nil {
			f.Close()
			if err == io.EOF {
				break
			}

			return tmpDir, errors.Wrap(err, "extracting image data")
		}
		f.Close()

		numFiles++
	}

	logrus.Infof("Successfully extracted %d files from image tarball %s", numFiles, tarPath)
	return tmpDir, err
}

// fix gosec G305: File traversal when extracting zip/tar archive
// more context: https://snyk.io/research/zip-slip-vulnerability
func sanitizeExtractPath(tmpDir, filePath string) (string, error) {
	destpath := filepath.Join(tmpDir, filePath)
	if !strings.HasPrefix(destpath, filepath.Clean(tmpDir)+string(os.PathSeparator)) {
		return "", fmt.Errorf("%s: illegal file path", filePath)
	}

	return destpath, nil
}

// readArchiveManifest extracts the manifest json from an image tar
//    archive and returns the data as a struct
func (di *spdxDefaultImplementation) ReadArchiveManifest(manifestPath string) (manifest *ArchiveManifest, err error) {
	// Check that we have the archive manifest.json file
	if !util.Exists(manifestPath) {
		return manifest, errors.New("unable to find manifest file " + manifestPath)
	}

	// Parse the json file
	manifestData := []ArchiveManifest{}
	manifestJSON, err := os.ReadFile(manifestPath)
	if err != nil {
		return manifest, errors.Wrap(err, "unable to read from tarfile")
	}
	if err := json.Unmarshal(manifestJSON, &manifestData); err != nil {
		fmt.Println(string(manifestJSON))
		return manifest, errors.Wrap(err, "unmarshalling image manifest")
	}
	return &manifestData[0], nil
}

// getImageReferences gets a reference string and returns all image
// references from it
func getImageReferences(referenceString string) ([]struct {
	Digest string
	Arch   string
	OS     string
}, error) {
	ref, err := name.ParseReference(referenceString)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing image reference %s", referenceString)
	}
	descr, err := remote.Get(ref)
	if err != nil {
		return nil, errors.Wrap(err, "fetching remote descriptor")
	}

	images := []struct {
		Digest string
		Arch   string
		OS     string
	}{}

	// If we got a digest, we reuse it as is
	if _, ok := ref.(name.Digest); ok {
		images = append(images, struct {
			Digest string
			Arch   string
			OS     string
		}{Digest: ref.(name.Digest).String()})
		return images, nil
	}

	// If the reference is not an image, it has to work as a tag
	tag, ok := ref.(name.Tag)
	if !ok {
		return nil, errors.Errorf("could not cast tag from reference %s", referenceString)
	}
	// If the reference points to an image, return it
	if descr.MediaType.IsImage() {
		logrus.Infof("Reference %s points to a single image", referenceString)
		// Check if we can get an image
		im, err := descr.Image()
		if err != nil {
			return nil, errors.Wrap(err, "getting image from descriptor")
		}

		imageDigest, err := im.Digest()
		if err != nil {
			return nil, errors.Wrap(err, "while calculating image digest")
		}

		dig, err := name.NewDigest(
			fmt.Sprintf(
				"%s/%s@%s:%s",
				tag.RegistryStr(), tag.RepositoryStr(),
				imageDigest.Algorithm, imageDigest.Hex,
			),
		)
		if err != nil {
			return nil, errors.Wrap(err, "building single image digest")
		}

		logrus.Infof("Adding image digest %s from reference", dig.String())
		return append(images, struct {
			Digest string
			Arch   string
			OS     string
		}{Digest: dig.String()}), nil
	}

	// Get the image index
	index, err := descr.ImageIndex()
	if err != nil {
		return nil, errors.Wrapf(err, "getting image index for %s", referenceString)
	}
	indexManifest, err := index.IndexManifest()
	if err != nil {
		return nil, errors.Wrapf(err, "getting index manifest from %s", referenceString)
	}
	logrus.Infof("Reference image index points to %d manifests", len(indexManifest.Manifests))

	for _, manifest := range indexManifest.Manifests {
		dig, err := name.NewDigest(
			fmt.Sprintf(
				"%s/%s@%s:%s",
				tag.RegistryStr(), tag.RepositoryStr(),
				manifest.Digest.Algorithm, manifest.Digest.Hex,
			))
		if err != nil {
			return nil, errors.Wrap(err, "generating digest for image")
		}

		logrus.Infof(
			"Adding image %s/%s@%s:%s (%s/%s)",
			tag.RegistryStr(), tag.RepositoryStr(), manifest.Digest.Algorithm, manifest.Digest.Hex,
			manifest.Platform.Architecture, manifest.Platform.OS,
		)
		arch, osid := "", ""
		if manifest.Platform != nil {
			arch = manifest.Platform.Architecture
			osid = manifest.Platform.OS
		}
		images = append(images,
			struct {
				Digest string
				Arch   string
				OS     string
			}{
				Digest: dig.String(),
				Arch:   arch,
				OS:     osid,
			})
	}
	return images, nil
}

func PullImageToArchive(referenceString, path string) error {
	ref, err := name.ParseReference(referenceString)
	if err != nil {
		return errors.Wrapf(err, "parsing reference %s", referenceString)
	}

	// Get the image from the reference:
	img, err := remote.Image(ref)
	if err != nil {
		return errors.Wrap(err, "getting image")
	}

	return errors.Wrap(tarball.WriteToFile(path, ref, img), "writing image to disk")
}

// PullImagesToArchive takes an image reference (a tag or a digest)
// and writes it into a docker tar archive in path
func (di *spdxDefaultImplementation) PullImagesToArchive(
	referenceString, path string,
) (images []struct {
	Reference string
	Archive   string
	Arch      string
	OS        string
}, err error) {
	images = []struct {
		Reference string
		Archive   string
		Arch      string
		OS        string
	}{}
	// Get the image references from the index
	references, err := getImageReferences(referenceString)
	if err != nil {
		return nil, err
	}

	if len(references) == 0 {
		return nil, errors.Wrap(err, "the supplied reference did not return any image references")
	}

	if !util.Exists(path) {
		if err := os.MkdirAll(path, os.FileMode(0o755)); err != nil {
			return nil, errors.Wrap(err, "creating image directory")
		}
	}

	for _, refData := range references {
		ref, err := name.ParseReference(refData.Digest)
		if err != nil {
			return nil, errors.Wrapf(err, "parsing reference %s", referenceString)
		}

		// Get the reference image
		img, err := remote.Image(ref)
		if err != nil {
			return nil, errors.Wrap(err, "getting image")
		}
		// This function is not for digests
		d, ok := ref.(name.Digest)
		if !ok {
			return nil, fmt.Errorf("reference is not a tag or digest")
		}
		p := strings.Split(d.DigestStr(), ":")
		tarPath := filepath.Join(path, p[1]+".tar")
		if err := tarball.MultiWriteToFile(
			tarPath,
			map[name.Tag]v1.Image{
				d.Repository.Tag(p[1]): img,
			},
		); err != nil {
			return nil, err
		}
		images = append(images, struct {
			Reference string
			Archive   string
			Arch      string
			OS        string
		}{refData.Digest, tarPath, refData.Arch, refData.OS})
	}
	return images, nil
}

// PackageFromTarball builds a SPDX package from the contents of a tarball
func (di *spdxDefaultImplementation) PackageFromTarball(
	opts *Options, tarOpts *TarballOptions, tarFile string,
) (pkg *Package, err error) {
	logrus.Infof("Generating SPDX package from tarball %s", tarFile)

	if tarOpts.AddFiles {
		// Estract the tarball
		tmp, err := di.ExtractTarballTmp(tarFile)
		if err != nil {
			return nil, errors.Wrap(err, "extracting tarball to temporary archive")
		}
		defer os.RemoveAll(tmp)
		pkg, err = di.PackageFromDirectory(opts, tmp)
		if err != nil {
			return nil, errors.Wrap(err, "generating package from tar contents")
		}
	} else {
		pkg = NewPackage()
	}
	// Set the extract dir option. This makes the package to remove
	// the tempdir prefix from the document paths:
	pkg.Options().WorkDir = tarOpts.ExtractDir
	if err := pkg.ReadSourceFile(tarFile); err != nil {
		return nil, errors.Wrapf(err, "reading source file %s", tarFile)
	}
	// Build the ID and the filename from the tarball name
	return pkg, nil
}

// GetDirectoryTree traverses a directory and return a slice of strings with all files
func (di *spdxDefaultImplementation) GetDirectoryTree(dirPath string) ([]string, error) {
	fileList := []string{}

	if err := fs.WalkDir(os.DirFS(dirPath), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		if d.Type() == os.ModeSymlink {
			return nil
		}

		fileList = append(fileList, path)
		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "buiding directory tree")
	}
	return fileList, nil
}

// IgnorePatterns return a list of gitignore patterns
func (di *spdxDefaultImplementation) IgnorePatterns(
	dirPath string, extraPatterns []string, skipGitIgnore bool,
) ([]gitignore.Pattern, error) {
	patterns := []gitignore.Pattern{}
	for _, s := range extraPatterns {
		patterns = append(patterns, gitignore.ParsePattern(s, nil))
	}

	if skipGitIgnore {
		logrus.Debug("Not using patterns in .gitignore")
		return patterns, nil
	}

	if util.Exists(filepath.Join(dirPath, gitIgnoreFile)) {
		f, err := os.Open(filepath.Join(dirPath, gitIgnoreFile))
		if err != nil {
			return nil, errors.Wrap(err, "opening gitignore file")
		}
		defer f.Close()

		// When using .gitignore files, we alwas add the .git directory
		// to match git's behavior
		patterns = append(patterns, gitignore.ParsePattern(".git/", nil))

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			s := scanner.Text()
			if !strings.HasPrefix(s, "#") && len(strings.TrimSpace(s)) > 0 {
				logrus.Debugf("Loaded .gitignore pattern: >>%s<<", s)
				patterns = append(patterns, gitignore.ParsePattern(s, nil))
			}
		}
	}

	logrus.Debugf(
		"Loaded %d patterns from .gitignore (+ %d extra) at root of directory", len(patterns), len(extraPatterns),
	)
	return patterns, nil
}

// ApplyIgnorePatterns applies the gitignore patterns to a list of files, removing matched
func (di *spdxDefaultImplementation) ApplyIgnorePatterns(
	fileList []string, patterns []gitignore.Pattern,
) (filteredList []string) {
	logrus.Infof(
		"Applying %d ignore patterns to list of %d filenames",
		len(patterns), len(fileList),
	)
	// We will return a new file list
	filteredList = []string{}

	// Build the new gitignore matcher
	matcher := gitignore.NewMatcher(patterns)

	// Cycle all files, removing those matched:
	for _, file := range fileList {
		if matcher.Match(strings.Split(file, string(filepath.Separator)), false) {
			logrus.Debugf("File ignored by .gitignore: %s", file)
		} else {
			filteredList = append(filteredList, file)
		}
	}
	return filteredList
}

// GetGoDependencies opens a Go module and directory and returns the
// dependencies as SPDX packages.
func (di *spdxDefaultImplementation) GetGoDependencies(
	path string, opts *Options,
) (spdxPackages []*Package, err error) {
	// Open the directory as a go module:
	mod, err := NewGoModuleFromPath(path)
	if err != nil {
		return nil, errors.Wrap(err, "creating a mod from the specified path")
	}
	mod.Options().OnlyDirectDeps = opts.OnlyDirectDeps
	mod.Options().ScanLicenses = opts.ScanLicenses

	// Open the module
	if err := mod.Open(); err != nil {
		return nil, errors.Wrap(err, "opening new module path")
	}

	defer func() { err = mod.RemoveDownloads() }()
	if opts.ScanLicenses {
		if errScan := mod.ScanLicenses(); err != nil {
			return nil, errScan
		}
	}

	spdxPackages = []*Package{}
	for _, goPkg := range mod.Packages {
		spdxPkg, err := goPkg.ToSPDXPackage()
		if err != nil {
			return nil, errors.Wrap(err, "converting go module to spdx package")
		}
		spdxPackages = append(spdxPackages, spdxPkg)
	}

	return spdxPackages, err
}

func (di *spdxDefaultImplementation) LicenseReader(spdxOpts *Options) (*license.Reader, error) {
	opts := license.DefaultReaderOptions
	opts.CacheDir = spdxOpts.LicenseCacheDir
	opts.LicenseDir = spdxOpts.LicenseData
	// Create the new reader
	reader, err := license.NewReaderWithOptions(opts)
	if err != nil {
		return nil, errors.Wrap(err, "creating reusable license reader")
	}
	return reader, nil
}

// GetDirectoryLicense takes a path and scans
// the files in it to determine licensins information
func (di *spdxDefaultImplementation) GetDirectoryLicense(
	reader *license.Reader, path string, spdxOpts *Options,
) (*license.License, error) {
	licenseResult, err := reader.ReadTopLicense(path)
	if err != nil {
		return nil, errors.Wrap(err, "getting directory license")
	}
	if licenseResult == nil {
		logrus.Warnf("License classifier could not find a license for directory: %v", err)
		return nil, nil
	}
	return licenseResult.License, nil
}

// ImageRefToPackage Returns a spdx package from an OCI image reference
func (di *spdxDefaultImplementation) ImageRefToPackage(ref string, opts *Options) (*Package, error) {
	tmpdir, err := os.MkdirTemp("", "doc-build-")
	if err != nil {
		return nil, errors.Wrap(err, "creating temporary workdir in")
	}
	imgs, err := di.PullImagesToArchive(ref, tmpdir)
	if err != nil {
		return nil, errors.Wrap(err, "while downloading images to archive")
	}

	if len(imgs) == 0 {
		return nil, errors.Errorf("Could not get any images from reference %s", ref)
	}

	// If we just got one image and that image is exactly the same
	// reference, return a single package:
	if len(imgs) == 1 && imgs[0].Reference == ref {
		return di.PackageFromImageTarball(opts, imgs[0].Archive)
	}

	// Create the package representing the image tag:
	pkg := &Package{}
	pkg.Name = ref
	pkg.BuildID(pkg.Name)
	pkg.DownloadLocation = ref

	// Now, cycle each image in the index and generate a package from it
	for _, img := range imgs {
		subpkg, err := di.PackageFromImageTarball(opts, img.Archive)
		if err != nil {
			return nil, errors.Wrap(err, "adding image variant package")
		}

		if img.Arch != "" || img.OS != "" {
			subpkg.Name = ref + " (" + img.Arch
			if img.Arch != "" {
				subpkg.Name += "/"
			}
			subpkg.Name += img.OS + ")"
		} else {
			subpkg.Name = img.Reference
		}
		subpkg.DownloadLocation = img.Reference

		// Add the package
		pkg.AddRelationship(&Relationship{
			Peer:       subpkg,
			Type:       CONTAINS,
			FullRender: true,
			Comment:    "Container image lager",
		})
		subpkg.AddRelationship(&Relationship{
			Peer:    pkg,
			Type:    VARIANT_OF,
			Comment: "Image index",
		})
	}
	return pkg, nil
}

// PackageFromImageTarball reads an OCI image archive and produces a SPDX
// packafe describing its layers
func (di *spdxDefaultImplementation) PackageFromImageTarball(
	spdxOpts *Options, tarPath string,
) (imagePackage *Package, err error) {
	logrus.Infof("Generating SPDX package from image tarball %s", tarPath)

	// Extract all files from tarfile
	tarOpts := &TarballOptions{}

	// If specified, add individual files from the tarball to the
	// spdx package, unless AnalyzeLayers is set because in that
	// case the individual analyzers decide to do that.
	if spdxOpts.AddTarFiles && !spdxOpts.AnalyzeLayers {
		tarOpts.AddFiles = true
	}
	tarOpts.ExtractDir, err = di.ExtractTarballTmp(tarPath)
	if err != nil {
		return nil, errors.Wrap(err, "extracting tarball to temp dir")
	}
	defer os.RemoveAll(tarOpts.ExtractDir)

	// Read the archive manifest json:
	manifest, err := di.ReadArchiveManifest(
		filepath.Join(tarOpts.ExtractDir, archiveManifestFilename),
	)
	if err != nil {
		return nil, errors.Wrap(err, "while reading docker archive manifest")
	}

	if len(manifest.RepoTags) == 0 {
		return nil, errors.New("No RepoTags found in manifest")
	}

	if manifest.RepoTags[0] == "" {
		return nil, errors.New(
			"unable to add tar archive, manifest does not have a RepoTags entry",
		)
	}

	logrus.Infof("Package describes %s image", manifest.RepoTags[0])

	// Create the new SPDX package
	imagePackage = NewPackage()
	imagePackage.Options().WorkDir = tarOpts.ExtractDir
	imagePackage.Name = manifest.RepoTags[0]
	imagePackage.BuildID(imagePackage.Name)

	logrus.Infof("Image manifest lists %d layers", len(manifest.LayerFiles))

	// Cycle all the layers from the manifest and add them as packages
	for _, layerFile := range manifest.LayerFiles {
		// Generate a package from a layer
		pkg, err := di.PackageFromTarball(spdxOpts, tarOpts, filepath.Join(tarOpts.ExtractDir, layerFile))
		if err != nil {
			return nil, errors.Wrap(err, "building package from layer")
		}
		// Regenerate the BuildID to avoid clashes when handling multiple
		// images at the same time.
		pkg.BuildID(manifest.RepoTags[0], layerFile)

		// If the option is enabled, scan the container layers
		if spdxOpts.AnalyzeLayers {
			if err := di.AnalyzeImageLayer(filepath.Join(tarOpts.ExtractDir, layerFile), pkg); err != nil {
				return nil, errors.Wrap(err, "scanning layer "+pkg.ID)
			}
		} else {
			logrus.Info("Not performing deep image analysis (opts.AnalyzeLayers = false)")
		}

		// Add the layer package to the image package
		if err := imagePackage.AddPackage(pkg); err != nil {
			return nil, errors.Wrap(err, "adding layer to image package")
		}
	}

	// return the finished package
	return imagePackage, nil
}

func (di *spdxDefaultImplementation) AnalyzeImageLayer(layerPath string, pkg *Package) error {
	return NewImageAnalyzer().AnalyzeLayer(layerPath, pkg)
}

// PackageFromDirectory scans a directory and returns its contents as a
// SPDX package, optionally determining the licenses found
func (di *spdxDefaultImplementation) PackageFromDirectory(opts *Options, dirPath string) (pkg *Package, err error) {
	dirPath, err = filepath.Abs(dirPath)
	if err != nil {
		return nil, errors.Wrap(err, "getting absolute directory path")
	}
	fileList, err := di.GetDirectoryTree(dirPath)
	if err != nil {
		return nil, errors.Wrap(err, "building directory tree")
	}
	reader, err := di.LicenseReader(opts)
	if err != nil {
		return nil, errors.Wrap(err, "creating license reader")
	}
	licenseTag := ""
	lic, err := di.GetDirectoryLicense(reader, dirPath, opts)
	if err != nil {
		return nil, errors.Wrap(err, "scanning directory for licenses")
	}
	if lic != nil {
		licenseTag = lic.LicenseID
	}

	// Build a list of patterns from those found in the .gitignore file and
	// posssibly others passed in the options:
	patterns, err := di.IgnorePatterns(
		dirPath, opts.IgnorePatterns, opts.NoGitignore,
	)
	if err != nil {
		return nil, errors.Wrap(err, "building ignore patterns list")
	}

	// Apply the ignore patterns to the list of files
	fileList = di.ApplyIgnorePatterns(fileList, patterns)
	logrus.Infof("Scanning %d files and adding them to the SPDX package", len(fileList))

	pkg = NewPackage()
	pkg.FilesAnalyzed = true
	pkg.Name = filepath.Base(dirPath)
	if pkg.Name == "" {
		pkg.Name = uuid.NewString()
	}
	pkg.LicenseConcluded = licenseTag

	// Set the working directory of the package:
	pkg.Options().WorkDir = filepath.Dir(dirPath)

	t := throttler.New(5, len(fileList))

	processDirectoryFile := func(path string, pkg *Package) {
		defer t.Done(err)
		f := NewFile()
		f.Options().WorkDir = dirPath
		f.Options().Prefix = pkg.Name

		lic, err = reader.LicenseFromFile(filepath.Join(dirPath, path))
		if err != nil {
			err = errors.Wrap(err, "scanning file for license")
			return
		}
		f.LicenseInfoInFile = NONE
		if lic == nil {
			f.LicenseConcluded = licenseTag
		} else {
			f.LicenseInfoInFile = lic.LicenseID
		}

		if err = f.ReadSourceFile(filepath.Join(dirPath, path)); err != nil {
			err = errors.Wrap(err, "checksumming file")
			return
		}
		if err = pkg.AddFile(f); err != nil {
			err = errors.Wrapf(err, "adding %s as file to the spdx package", path)
			return
		}
	}

	// Read the files in parallel
	for _, path := range fileList {
		go processDirectoryFile(path, pkg)
		t.Throttle()
	}

	// If the throttler picked an error, fail here
	if err := t.Err(); err != nil {
		return nil, err
	}

	// Add files into the package
	return pkg, nil
}
