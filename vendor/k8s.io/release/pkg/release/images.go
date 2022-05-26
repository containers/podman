/*
Copyright 2020 The Kubernetes Authors.

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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"sigs.k8s.io/release-utils/command"
)

// Images is a wrapper around container image related functionality.
type Images struct {
	imageImpl
}

// NewImages creates a new Images instance
func NewImages() *Images {
	return &Images{&defaultImageImpl{}}
}

// SetImpl can be used to set the internal image implementation.
func (i *Images) SetImpl(impl imageImpl) {
	i.imageImpl = impl
}

// imageImpl is a client for working with container images.
//counterfeiter:generate . imageImpl
type imageImpl interface {
	Execute(cmd string, args ...string) error
	ExecuteOutput(cmd string, args ...string) (string, error)
	RepoTagFromTarball(path string) (string, error)
}

type defaultImageImpl struct{}

func (*defaultImageImpl) Execute(cmd string, args ...string) error {
	return command.New(cmd, args...).RunSilentSuccess()
}

func (*defaultImageImpl) ExecuteOutput(cmd string, args ...string) (string, error) {
	res, err := command.New(cmd, args...).RunSilentSuccessOutput()
	if err != nil {
		return "", err
	}
	return res.OutputTrimNL(), nil
}

func (*defaultImageImpl) RepoTagFromTarball(path string) (string, error) {
	tagOutput, err := command.
		New("tar", "xf", path, "manifest.json", "-O").
		Pipe("jq", "-r", ".[0].RepoTags[0]").
		RunSilentSuccessOutput()
	if err != nil {
		return "", err
	}
	return tagOutput.OutputTrimNL(), nil
}

var tagRegex = regexp.MustCompile(`^.+/(.+):.+$`)

// PublishImages releases container images to the provided target registry
func (i *Images) Publish(registry, version, buildPath string) error {
	version = i.normalizeVersion(version)

	releaseImagesPath := filepath.Join(buildPath, ImagesPath)
	logrus.Infof(
		"Pushing container images from %s to registry %s",
		releaseImagesPath, registry,
	)

	manifestImages, err := i.GetManifestImages(
		registry, version, buildPath,
		func(path, origTag, newTagWithArch string) error {
			if err := i.Execute(
				"docker", "load", "-qi", path,
			); err != nil {
				return errors.Wrap(err, "load container image")
			}

			if err := i.Execute(
				"docker", "tag", origTag, newTagWithArch,
			); err != nil {
				return errors.Wrap(err, "tag container image")
			}

			logrus.Infof("Pushing %s", newTagWithArch)

			if err := i.Execute(
				"gcloud", "docker", "--", "push", newTagWithArch,
			); err != nil {
				return errors.Wrap(err, "push container image")
			}

			if err := i.Execute(
				"docker", "rmi", origTag, newTagWithArch,
			); err != nil {
				return errors.Wrap(err, "remove local container image")
			}

			return nil
		},
	)
	if err != nil {
		return errors.Wrap(err, "get manifest images")
	}

	if err := os.Setenv("DOCKER_CLI_EXPERIMENTAL", "enabled"); err != nil {
		return errors.Wrap(err, "enable docker experimental CLI")
	}

	for image, arches := range manifestImages {
		imageVersion := fmt.Sprintf("%s:%s", image, version)
		logrus.Infof("Creating manifest image %s", imageVersion)

		manifests := []string{}
		for _, arch := range arches {
			manifests = append(manifests,
				fmt.Sprintf("%s-%s:%s", image, arch, version),
			)
		}
		if err := i.Execute("docker", append(
			[]string{"manifest", "create", "--amend", imageVersion},
			manifests...,
		)...); err != nil {
			return errors.Wrap(err, "create manifest")
		}

		for _, arch := range arches {
			logrus.Infof(
				"Annotating %s-%s:%s with --arch %s",
				image, arch, version, arch,
			)
			if err := i.Execute(
				"docker", "manifest", "annotate", "--arch", arch,
				imageVersion, fmt.Sprintf("%s-%s:%s", image, arch, version),
			); err != nil {
				return errors.Wrap(err, "annotate manifest with arch")
			}
		}

		logrus.Infof("Pushing manifest image %s", imageVersion)
		if err := i.Execute(
			"docker", "manifest", "push", imageVersion, "--purge",
		); err != nil {
			return errors.Wrap(err, "push manifest")
		}
	}

	return nil
}

// Validates that image manifests have been pushed to a specified remote
// registry.
func (i *Images) Validate(registry, version, buildPath string) error {
	logrus.Infof("Validating image manifests in %s", registry)
	version = i.normalizeVersion(version)

	manifestImages, err := i.GetManifestImages(
		registry, version, buildPath, nil,
	)
	if err != nil {
		return errors.Wrap(err, "get manifest images")
	}
	logrus.Infof("Got manifest images %+v", manifestImages)

	for image, arches := range manifestImages {
		imageVersion := fmt.Sprintf("%s:%s", image, version)

		manifestBytes, err := crane.Manifest(imageVersion)
		if err != nil {
			return errors.Wrapf(
				err, "get remote manifest from %s", imageVersion,
			)
		}

		manifest := string(manifestBytes)
		manifestFile, err := os.CreateTemp("", "manifest-")
		if err != nil {
			return errors.Wrap(err, "create temp file for manifest")
		}
		if _, err := manifestFile.WriteString(manifest); err != nil {
			return errors.Wrapf(
				err, "write manifest to %s", manifestFile.Name(),
			)
		}
		defer os.RemoveAll(manifestFile.Name())

		for _, arch := range arches {
			logrus.Infof(
				"Checking image digest for %s on %s architecture", image, arch,
			)

			digest, err := i.ExecuteOutput(
				"jq", "--arg", "a", arch, "-r",
				".manifests[] | select(.platform.architecture == $a) | .digest",
				manifestFile.Name(),
			)
			if err != nil {
				return errors.Wrapf(
					err, "get digest from manifest file %s for arch %s",
					manifestFile.Name(), arch,
				)
			}

			if digest == "" {
				return errors.Errorf(
					"could not find the image digest for %s on %s",
					imageVersion, arch,
				)
			}

			logrus.Infof("Digest for %s on %s: %s", imageVersion, arch, digest)
		}
	}

	return nil
}

// Exists verifies that a set of image manifests exists on a specified remote
// registry. This is a simpler check than Validate, which doesn't presuppose the
// existence of a local build directory. Used in CI builds to quickly validate
// if a build is actually required.
func (i *Images) Exists(registry, version string, fast bool) (bool, error) {
	logrus.Infof("Validating image manifests in %s", registry)
	version = i.normalizeVersion(version)

	manifestImages := ManifestImages

	arches := SupportedArchitectures
	if fast {
		arches = FastArchitectures
	}

	for _, image := range manifestImages {
		imageVersion := fmt.Sprintf("%s/%s:%s", registry, image, version)

		manifestBytes, err := crane.Manifest(imageVersion)
		if err != nil {
			return false, errors.Wrapf(
				err, "get remote manifest from %s", imageVersion,
			)
		}

		manifest := string(manifestBytes)
		manifestFile, err := os.CreateTemp("", "manifest-")
		if err != nil {
			return false, errors.Wrap(err, "create temp file for manifest")
		}
		if _, err := manifestFile.WriteString(manifest); err != nil {
			return false, errors.Wrapf(
				err, "write manifest to %s", manifestFile.Name(),
			)
		}
		defer os.RemoveAll(manifestFile.Name())

		for _, arch := range arches {
			logrus.Infof(
				"Checking image digest for %s on %s architecture", image, arch,
			)

			digest, err := i.ExecuteOutput(
				"jq", "--arg", "a", arch, "-r",
				".manifests[] | select(.platform.architecture == $a) | .digest",
				manifestFile.Name(),
			)
			if err != nil {
				return false, errors.Wrapf(
					err, "get digest from manifest file %s for arch %s",
					manifestFile.Name(), arch,
				)
			}

			if digest == "" {
				return false, errors.Errorf(
					"could not find the image digest for %s on %s",
					imageVersion, arch,
				)
			}

			logrus.Infof("Digest for %s on %s: %s", imageVersion, arch, digest)
		}
	}

	return true, nil
}

// GetManifestImages can be used to retrieve the map of built images and
// architectures.
func (i *Images) GetManifestImages(
	registry, version, buildPath string,
	forTarballFn func(path, origTag, newTagWithArch string) error,
) (map[string][]string, error) {
	manifestImages := make(map[string][]string)

	releaseImagesPath := filepath.Join(buildPath, ImagesPath)
	logrus.Infof("Getting manifest images in %s", releaseImagesPath)

	archPaths, err := os.ReadDir(releaseImagesPath)
	if err != nil {
		return nil, errors.Wrapf(err, "read images path %s", releaseImagesPath)
	}

	for _, archPath := range archPaths {
		arch := archPath.Name()
		if !archPath.IsDir() {
			logrus.Infof("Skipping %s because it's not a directory", arch)
			continue
		}

		if err := filepath.Walk(
			filepath.Join(releaseImagesPath, arch),
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					return nil
				}

				fileName := info.Name()
				if !strings.HasSuffix(fileName, ".tar") {
					logrus.Infof("Skipping non-tarball %s", fileName)
					return nil
				}

				origTag, err := i.RepoTagFromTarball(path)
				if err != nil {
					return errors.Wrap(err, "getting repo tags for tarball")
				}

				tagMatches := tagRegex.FindStringSubmatch(origTag)
				if len(tagMatches) != 2 {
					return errors.Errorf(
						"malformed tag %s in %s", origTag, path,
					)
				}

				binary := tagMatches[1]
				newTag := filepath.Join(
					registry,
					strings.TrimSuffix(binary, "-"+arch),
				)
				newTagWithArch := fmt.Sprintf("%s-%s:%s", newTag, arch, version)
				manifestImages[newTag] = append(manifestImages[newTag], arch)

				if forTarballFn != nil {
					if err := forTarballFn(
						path, origTag, newTagWithArch,
					); err != nil {
						return errors.Wrap(err, "executing tarball callback")
					}
				}
				return nil
			},
		); err != nil {
			return nil, errors.Wrap(err, "traversing path")
		}
	}
	return manifestImages, nil
}

// normalizeVersion normalizes an container image version by replacing all invalid characters.
func (i *Images) normalizeVersion(version string) string {
	return strings.ReplaceAll(version, "+", "_")
}
