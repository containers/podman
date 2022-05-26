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

package release

import (
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/release/pkg/binary"
)

type ArtifactChecker struct {
	opts *ArtifactCheckerOptions
	impl artifactCheckerImplementation
}

type ArtifactCheckerOptions struct {
	GitRoot  string   // Directory where the repo was cloned
	Versions []string // Version tags we are checking
}

func NewArtifactChecker() *ArtifactChecker {
	return NewArtifactCheckerWithOptions(&ArtifactCheckerOptions{})
}

func NewArtifactCheckerWithOptions(opts *ArtifactCheckerOptions) *ArtifactChecker {
	return &ArtifactChecker{
		opts: opts,
		impl: &defaultArtifactCheckerImpl{},
	}
}

func (ac *ArtifactChecker) Options() *ArtifactCheckerOptions {
	return ac.opts
}

// CheckBinaryTags checks that the binaries produced in the release are
// correctly tagged with the semver string
func (ac *ArtifactChecker) CheckBinaryTags() error {
	for _, tag := range ac.opts.Versions {
		if err := ac.impl.CheckVersionTags(ac.opts, tag); err != nil {
			return errors.Wrapf(err, "checking tags in %s binaries", tag)
		}
	}
	return nil
}

// CheckBinaryArchitectures ensures all the artifacts produced in each
// release are of the right architecture
func (ac *ArtifactChecker) CheckBinaryArchitectures() error {
	for _, tag := range ac.opts.Versions {
		if err := ac.impl.CheckVersionArch(ac.opts, tag); err != nil {
			return errors.Wrapf(err, "checking tags in %s binaries", tag)
		}
	}
	return nil
}

type artifactCheckerImplementation interface {
	ListReleaseBinaries(opts *ArtifactCheckerOptions, version string) ([]struct{ Path, Platform, Arch string }, error)
	CheckVersionTags(*ArtifactCheckerOptions, string) error
	CheckVersionArch(*ArtifactCheckerOptions, string) error
}

type defaultArtifactCheckerImpl struct{}

// ListReleaseBinaries lists a release's binaries, with expected platform
func (impl *defaultArtifactCheckerImpl) ListReleaseBinaries(
	opts *ArtifactCheckerOptions, version string,
) (
	list []struct{ Path, Platform, Arch string }, err error,
) {
	return ListBuildBinaries(opts.GitRoot, version)
}

// CheckVersionTags checks the binaries of a release to verify they have
// the correct version tag
func (impl *defaultArtifactCheckerImpl) CheckVersionTags(
	opts *ArtifactCheckerOptions, version string,
) error {
	binaries, err := impl.ListReleaseBinaries(opts, version)
	if err != nil {
		return errors.Wrapf(err, "listing binaries for release %s", version)
	}
	logrus.Infof("Checking %d binaries for tag %s", len(binaries), version)
	for _, binData := range binaries {
		bin, err := binary.New(binData.Path)
		if err != nil {
			return errors.Wrapf(err, "creating binary from %s", binData.Path)
		}

		// The mounter binary is not tagged
		if filepath.Base(binData.Path) == "mounter" {
			continue
		}

		// TODO: Ensure binary contains the correct commit message
		contains, err := bin.ContainsStrings(version)
		if err != nil {
			return errors.Wrapf(err, "scanning binary %s", binData.Path)
		}
		if !contains {
			return errors.Errorf(
				"tag %s not found in produced binary: %s ", version, binData.Path,
			)
		}
	}
	return nil
}

// CheckVersionArch checks that the binaries of a certain version are
// in fact of the expected OS/Arch.
func (impl *defaultArtifactCheckerImpl) CheckVersionArch(
	opts *ArtifactCheckerOptions, version string,
) error {
	binaries, err := impl.ListReleaseBinaries(opts, version)
	if err != nil {
		return errors.Wrapf(err, "listing binaries for release %s", version)
	}
	logrus.Infof("Ensuring architecture of %d binaries for version %s", len(binaries), version)
	for _, binData := range binaries {
		bin, err := binary.New(binData.Path)
		if err != nil {
			return errors.Wrapf(err, "creating binary object from %s", binData.Path)
		}

		if bin.Arch() != binData.Arch || bin.OS() != binData.Platform {
			return errors.Errorf(
				"binary %s has incorrect architecture: expected %s/%s got %s/%s",
				binData.Path, binData.Arch, binData.Platform, bin.Arch(), bin.OS(),
			)
		}
	}
	return nil
}
