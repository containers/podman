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
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"sigs.k8s.io/bom/pkg/license"
	"sigs.k8s.io/bom/pkg/spdx"
	"sigs.k8s.io/release-sdk/git"
	"sigs.k8s.io/release-sdk/github"
	"sigs.k8s.io/release-sdk/object"
	"sigs.k8s.io/release-utils/tar"
	"sigs.k8s.io/release-utils/util"
)

// PrepareWorkspaceStage sets up the workspace by cloning a new copy of k/k.
func PrepareWorkspaceStage(directory string, noMock bool) error {
	logrus.Infof("Preparing workspace for staging in %s", directory)

	k8sOrg := GetK8sOrg()
	k8sRepo := GetK8sRepo()
	isDefaultK8sUpstream := IsDefaultK8sUpstream()

	if noMock && !isDefaultK8sUpstream {
		// TODO: We could allow releasing custom Kubernetes forks by pushing
		// the artifacts into non-default locations. This needs further
		// investigation and goes beyond the currently implemented testing
		// approach.
		return errors.Errorf(
			"staging non default upstream Kubernetes is forbidden. " +
				"Verify that the $K8S_ORG, $K8S_REPO and $K8S_REF " +
				"environment variables point to their defaults when " +
				"doing using nomock releases.",
		)
	}

	logrus.Infof("Cloning repository %s/%s to %s", k8sOrg, k8sRepo, directory)

	repo, err := git.CloneOrOpenGitHubRepo(directory, k8sOrg, k8sRepo, false)
	if err != nil {
		return errors.Wrap(err, "clone k/k repository")
	}

	// Prewarm the SPDX licenses cache. As it is one of the main
	// remote operations, we do it now to have the data and fail early
	// is something goes wrong.
	s := spdx.NewSPDX()
	logrus.Infof("Caching SPDX license set to %s", s.Options().LicenseCacheDir)
	doptions := license.DefaultDownloaderOpts
	doptions.CacheDir = s.Options().LicenseCacheDir
	downloader, err := license.NewDownloaderWithOptions(doptions)
	if err != nil {
		return errors.Wrap(err, "creating license downloader")
	}
	// Fetch the SPDX licenses
	if _, err := downloader.GetLicenses(); err != nil {
		return errors.Wrap(err, "retrieving SPDX licenses")
	}

	if isDefaultK8sUpstream {
		token, ok := os.LookupEnv(github.TokenEnvKey)
		if !ok {
			return errors.Errorf("%s env variable is not set", github.TokenEnvKey)
		}

		if err := repo.SetURL(git.DefaultRemote, (&url.URL{
			Scheme: "https",
			User:   url.UserPassword("git", token),
			Host:   "github.com",
			Path:   filepath.Join(git.DefaultGithubOrg, git.DefaultGithubRepo),
		}).String()); err != nil {
			return errors.Wrap(err, "changing git remote of repository")
		}
	} else {
		logrus.Info("Using non-default k8s upstream, doing no git modifications")
	}

	return nil
}

// PrepareWorkspaceRelease sets up the workspace by downloading and extracting
// the staged sources on the provided bucket.
func PrepareWorkspaceRelease(directory, buildVersion, bucket string) error {
	logrus.Infof("Preparing workspace for release in %s", directory)
	logrus.Infof("Searching for staged %s on %s", SourcesTar, bucket)
	tempDir, err := os.MkdirTemp("", "staged-")
	if err != nil {
		return errors.Wrap(err, "create staged sources temp dir")
	}
	defer os.RemoveAll(tempDir)

	// On `release`, we lookup the staged sources and use them directly
	src := filepath.Join(bucket, StagePath, buildVersion, SourcesTar)
	dst := filepath.Join(tempDir, SourcesTar)

	gcs := object.NewGCS()
	gcs.WithAllowMissing(false)
	if err := gcs.CopyToLocal(src, dst); err != nil {
		return errors.Wrap(err, "copying staged sources from GCS")
	}

	logrus.Info("Got staged sources, extracting archive")
	if err := tar.Extract(
		dst, strings.TrimSuffix(directory, "/src/k8s.io/kubernetes"),
	); err != nil {
		return errors.Wrapf(err, "extracting %s", dst)
	}

	// Reset the github token in the staged k/k clone
	token, ok := os.LookupEnv(github.TokenEnvKey)
	if !ok {
		return errors.Errorf("%s env variable is not set", github.TokenEnvKey)
	}

	repo, err := git.OpenRepo(directory)
	if err != nil {
		return errors.Wrap(err, "opening staged clone of k/k")
	}

	if err := repo.SetURL(git.DefaultRemote, (&url.URL{
		Scheme: "https",
		User:   url.UserPassword("git", token),
		Host:   "github.com",
		Path:   filepath.Join(git.DefaultGithubOrg, git.DefaultGithubRepo),
	}).String()); err != nil {
		return errors.Wrap(err, "changing git remote of repository")
	}

	return nil
}

// ListBuildBinaries returns a list of binaries
func ListBuildBinaries(gitroot, version string) (list []struct{ Path, Platform, Arch string }, err error) {
	list = []struct {
		Path     string
		Platform string
		Arch     string
	}{}
	buildDir := filepath.Join(
		gitroot, fmt.Sprintf("%s-%s", BuildDir, version),
	)

	rootPath := filepath.Join(buildDir, ReleaseStagePath)
	platformsPath := filepath.Join(rootPath, "client")
	if !util.Exists(platformsPath) {
		logrus.Infof("Not adding binaries as %s was not found", platformsPath)
		return list, nil
	}
	platformsAndArches, err := os.ReadDir(platformsPath)
	if err != nil {
		return nil, errors.Wrapf(err, "retrieve platforms from %s", platformsPath)
	}

	for _, platformArch := range platformsAndArches {
		if !platformArch.IsDir() {
			logrus.Warnf(
				"Skipping platform and arch %q because it's not a directory",
				platformArch.Name(),
			)
			continue
		}

		split := strings.Split(platformArch.Name(), "-")
		if len(split) != 2 {
			return nil, errors.Errorf(
				"expected `platform-arch` format for %s", platformArch.Name(),
			)
		}

		platform := split[0]
		arch := split[1]

		src := filepath.Join(
			rootPath, "client", platformArch.Name(), "kubernetes", "client", "bin",
		)

		// We assume here the "server package" is a superset of the "client
		// package"
		serverSrc := filepath.Join(rootPath, "server", platformArch.Name())
		if util.Exists(serverSrc) {
			src = filepath.Join(serverSrc, "kubernetes", "server", "bin")
		}

		if err := filepath.Walk(src,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					return nil
				}

				// The binaries directory stores the image tarfiles and the
				// docker tag files. Skip those from the binaries list
				if strings.HasSuffix(path, ".docker_tag") || strings.HasSuffix(path, ".tar") {
					return nil
				}

				list = append(list, struct {
					Path     string
					Platform string
					Arch     string
				}{path, platform, arch})
				return nil
			},
		); err != nil {
			return nil, errors.Wrapf(err, "gathering binaries from %s", src)
		}

		// Copy node binaries if they exist and this isn't a 'server' platform
		nodeSrc := filepath.Join(rootPath, "node", platformArch.Name())
		if !util.Exists(serverSrc) && util.Exists(nodeSrc) {
			src = filepath.Join(nodeSrc, "kubernetes", "node", "bin")
			if err := filepath.Walk(src,
				func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if info.IsDir() {
						return nil
					}

					list = append(list, struct {
						Path     string
						Platform string
						Arch     string
					}{path, platform, arch})
					return nil
				},
			); err != nil {
				return nil, errors.Wrapf(err, "gathering node binaries from %s", src)
			}
		}
	}
	return list, nil
}

// ListBuildTarballs returns a list of the client, node server and other tarballs
func ListBuildTarballs(gitroot, version string) (tarList []string, err error) {
	tarsPath := filepath.Join(
		gitroot, fmt.Sprintf("%s-%s", BuildDir, version), ReleaseTarsPath,
	)

	tarList = []string{}
	if err := filepath.Walk(tarsPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}

			if strings.HasSuffix(path, "tar.gz") {
				tarList = append(tarList, path)
			}
			return nil
		},
	); err != nil {
		return nil, errors.Wrapf(err, "gathering tarfiles binaries from %s", tarsPath)
	}
	return tarList, nil
}

// ListBuildImages returns a slice with paths to all images produced by the build
func ListBuildImages(gitroot, version string) (imageList []string, err error) {
	imageList = []string{}
	buildDir := filepath.Join(
		gitroot, fmt.Sprintf("%s-%s", BuildDir, version),
	)

	arches, err := os.ReadDir(filepath.Join(buildDir, ImagesPath))
	if err != nil {
		return nil, errors.Wrap(err, "opening images directory")
	}
	for _, arch := range arches {
		if !arch.IsDir() {
			continue
		}
		images, err := os.ReadDir(filepath.Join(buildDir, ImagesPath, arch.Name()))
		if err != nil {
			return nil, errors.Wrapf(err, "opening %s images directory", arch.Name())
		}
		for _, tarball := range images {
			imageList = append(
				imageList, filepath.Join(buildDir, ImagesPath, arch.Name(), tarball.Name()),
			)
		}
	}
	return imageList, nil
}
