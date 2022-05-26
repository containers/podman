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

package object

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"sigs.k8s.io/release-sdk/gcli"
)

type GCS struct {
	// gsutil options
	concurrent bool
	recursive  bool
	noClobber  bool

	// local options
	// AllowMissing allows a copy operation to be skipped if the source or
	// destination does not exist. This is useful for scenarios where copy
	// operations happen in a loop/channel, so a single "failure" does not block
	// the entire operation.
	allowMissing bool
}

func NewGCS() *GCS {
	return &GCS{
		concurrent:   true,
		recursive:    true,
		noClobber:    true,
		allowMissing: true,
	}
}

func (g *GCS) SetOptions(opts ...OptFn) {
	for _, f := range opts {
		f(g)
	}
}

func (g *GCS) WithConcurrent(concurrent bool) OptFn {
	return func(Store) {
		g.concurrent = concurrent
	}
}

func (g *GCS) WithRecursive(recursive bool) OptFn {
	return func(Store) {
		g.recursive = recursive
	}
}

func (g *GCS) WithNoClobber(noClobber bool) OptFn {
	return func(Store) {
		g.noClobber = noClobber
	}
}

func (g *GCS) WithAllowMissing(allowMissing bool) OptFn {
	return func(Store) {
		g.allowMissing = allowMissing
	}
}

func (g *GCS) Concurrent() bool {
	return g.concurrent
}

func (g *GCS) Recursive() bool {
	return g.recursive
}

func (g *GCS) NoClobber() bool {
	return g.noClobber
}

func (g *GCS) AllowMissing() bool {
	return g.allowMissing
}

var (
	// GcsPrefix url prefix for google cloud storage buckets
	GcsPrefix      = "gs://"
	concurrentFlag = "-m"
	recursiveFlag  = "-r"
	noClobberFlag  = "-n"
)

// CopyToRemote copies a local directory to the specified GCS path
func (g *GCS) CopyToRemote(src, gcsPath string) error {
	logrus.Infof("Copying %s to GCS (%s)", src, gcsPath)
	gcsPath, gcsPathErr := g.NormalizePath(gcsPath)
	if gcsPathErr != nil {
		return errors.Wrap(gcsPathErr, "normalize GCS path")
	}

	_, err := os.Stat(src)
	if err != nil {
		logrus.Info("Unable to get local source directory info")

		if g.allowMissing {
			logrus.Infof("Source directory (%s) does not exist. Skipping GCS upload.", src)
			return nil
		}

		return errors.New("source directory does not exist")
	}

	return g.bucketCopy(src, gcsPath)
}

// CopyToLocal copies a GCS path to the specified local directory
func (g *GCS) CopyToLocal(gcsPath, dst string) error {
	logrus.Infof("Copying GCS (%s) to %s", gcsPath, dst)
	gcsPath, gcsPathErr := g.NormalizePath(gcsPath)
	if gcsPathErr != nil {
		return errors.Wrap(gcsPathErr, "normalize GCS path")
	}

	return g.bucketCopy(gcsPath, dst)
}

// CopyBucketToBucket copies between two GCS paths.
func (g *GCS) CopyBucketToBucket(src, dst string) error {
	logrus.Infof("Copying %s to %s", src, dst)

	src, srcErr := g.NormalizePath(src)
	if srcErr != nil {
		return errors.Wrap(srcErr, "normalize GCS path")
	}

	dst, dstErr := g.NormalizePath(dst)
	if dstErr != nil {
		return errors.Wrap(dstErr, "normalize GCS path")
	}

	return g.bucketCopy(src, dst)
}

func (g *GCS) bucketCopy(src, dst string) error {
	args := []string{}

	if g.concurrent {
		logrus.Debug("Setting GCS copy to run concurrently")
		args = append(args, concurrentFlag)
	}

	args = append(args, "cp")
	if g.recursive {
		logrus.Debug("Setting GCS copy to run recursively")
		args = append(args, recursiveFlag)
	}
	if g.noClobber {
		logrus.Debug("Setting GCS copy to not clobber existing files")
		args = append(args, noClobberFlag)
	}

	args = append(args, src, dst)

	if err := gcli.GSUtil(args...); err != nil {
		return errors.Wrap(err, "gcs copy")
	}

	return nil
}

// GetReleasePath returns a GCS path to retrieve builds from or push builds to
//
// Expected destination format:
//   gs://<bucket>/<gcsRoot>[/fast][/<version>]
func (g *GCS) GetReleasePath(
	bucket, gcsRoot, version string,
	fast bool) (string, error) {
	gcsPath, err := g.getPath(
		bucket,
		gcsRoot,
		version,
		"release",
		fast,
	)
	if err != nil {
		return "", errors.Wrap(err, "normalize GCS path")
	}

	logrus.Infof("Release path is %s", gcsPath)
	return gcsPath, nil
}

// GetMarkerPath returns a GCS path where version markers should be stored
//
// Expected destination format:
//   gs://<bucket>/<gcsRoot>
func (g *GCS) GetMarkerPath(
	bucket, gcsRoot string) (string, error) {
	gcsPath, err := g.getPath(
		bucket,
		gcsRoot,
		"",
		"marker",
		false,
	)
	if err != nil {
		return "", errors.Wrap(err, "normalize GCS path")
	}

	logrus.Infof("Version marker path is %s", gcsPath)
	return gcsPath, nil
}

// GetReleasePath returns a GCS path to retrieve builds from or push builds to
//
// Expected destination format:
//   gs://<bucket>/<gcsRoot>[/fast][/<version>]
// TODO: Support "release" buildType
func (g *GCS) getPath(
	bucket, gcsRoot, version, pathType string,
	fast bool) (string, error) {
	if gcsRoot == "" {
		return "", errors.New("GCS root must be specified")
	}

	gcsPathParts := []string{}

	gcsPathParts = append(gcsPathParts, bucket, gcsRoot)

	switch pathType {
	case "release":
		if fast {
			gcsPathParts = append(gcsPathParts, "fast")
		}

		if version != "" {
			gcsPathParts = append(gcsPathParts, version)
		}
	case "marker":
	default:
		return "", errors.New("a GCS path type must be specified")
	}

	// Ensure any constructed GCS path is prefixed with `gs://`
	return g.NormalizePath(gcsPathParts...)
}

// NormalizePath takes a GCS path and ensures that the `GcsPrefix` is
// prepended to it.
// TODO: Should there be an append function for paths to prevent multiple calls
//       like in build.checkBuildExists()?
func (g *GCS) NormalizePath(gcsPathParts ...string) (string, error) {
	gcsPath := ""

	// Ensure there is at least one element in the gcsPathParts slice before
	// trying to construct a path
	switch len(gcsPathParts) {
	case 0:
		return "", errors.New("must contain at least one path part")
	case 1:
		if gcsPathParts[0] == "" {
			return "", errors.New("path should not be an empty string")
		}

		gcsPath = gcsPathParts[0]
	default:
		var emptyParts int

		for i, part := range gcsPathParts {
			if part == "" {
				emptyParts++
			}

			if i == 0 {
				continue
			}

			if strings.Contains(part, "gs:/") {
				return "", errors.New("one of the GCS path parts contained a `gs:/`, which may suggest a filepath.Join() error in the caller")
			}

			if i == len(gcsPathParts)-1 && emptyParts == len(gcsPathParts) {
				return "", errors.New("all paths provided were empty")
			}
		}

		gcsPath = filepath.Join(gcsPathParts...)
	}

	// Strip `gs://` if it was included in gcsPathParts
	gcsPath = strings.TrimPrefix(gcsPath, GcsPrefix)

	// Strip `gs:/` if:
	// - `gs://` was included in gcsPathParts
	// - gcsPathParts had more than element
	// - filepath.Join() was called somewhere in a caller's logic
	gcsPath = strings.TrimPrefix(gcsPath, "gs:/")

	// Strip `/`
	// This scenario may never happen, but let's catch it, just in case
	gcsPath = strings.TrimPrefix(gcsPath, "/")

	gcsPath = GcsPrefix + gcsPath

	isNormalized := g.IsPathNormalized(gcsPath)
	if !isNormalized {
		return gcsPath, errors.New("unknown error while trying to normalize GCS path")
	}

	return gcsPath, nil
}

// IsPathNormalized determines if a GCS path is prefixed with `gs://`.
// Use this function as pre-check for any gsutil/GCS functions that manipulate
// GCS bucket contents.
func (g *GCS) IsPathNormalized(gcsPath string) bool {
	var errCount int

	if !strings.HasPrefix(gcsPath, GcsPrefix) {
		logrus.Errorf(
			"GCS path (%s) should be prefixed with `%s`", gcsPath, GcsPrefix,
		)
		errCount++
	}

	strippedPath := strings.TrimPrefix(gcsPath, GcsPrefix)
	if strings.Contains(strippedPath, "gs:/") {
		logrus.Errorf("GCS path (%s) should be prefixed with `gs:/`", gcsPath)
		errCount++
	}

	// TODO: Add logic to handle invalid path characters

	if errCount > 0 {
		return false
	}

	return true
}

// RsyncRecursive runs `gsutil rsync` in recursive mode. The caller of this
// function has to ensure that the provided paths are prefixed with gs:// if
// necessary (see `NormalizePath()`).
func (g *GCS) RsyncRecursive(src, dst string) error {
	return errors.Wrap(
		gcli.GSUtil(concurrentFlag, "rsync", recursiveFlag, src, dst),
		"running gsutil rsync",
	)
}

// PathExists returns true if the specified GCS path exists.
func (g *GCS) PathExists(gcsPath string) (bool, error) {
	if !g.IsPathNormalized(gcsPath) {
		return false, errors.Errorf(
			"cannot run `gsutil ls` GCS path does not begin with `%s`",
			GcsPrefix,
		)
	}

	// Do an ls with gsutil to check if the file exists:
	if err := gcli.GSUtil(
		"ls",
		gcsPath,
	); err != nil {
		// .. but check the message because if not found
		// it will exit with an error
		if strings.Contains(err.Error(), "One or more URLs matched no objects") {
			return false, nil
		}
		// Anything else we treat as error
		return false, err
	}

	logrus.Infof("Found %s", gcsPath)
	return true, nil
}

// DeletePath deletes a bucket location recursively
func (g *GCS) DeletePath(path string) error {
	path, err := g.NormalizePath(path)
	if err != nil {
		return errors.Wrap(err, "normalizing GCS path")
	}

	// Build the command arguments
	args := []string{"-q"}

	if g.concurrent {
		logrus.Debug("Setting GCS copy to run concurrently")
		args = append(args, concurrentFlag)
	}

	args = append(args, "rm")

	if g.recursive {
		logrus.Debug("Setting GCS copy to run recursively")
		args = append(args, recursiveFlag)
	}

	args = append(args, path)

	// Call gsutil to remove the path
	if err = gcli.GSUtil(args...); err != nil {
		return errors.Wrap(err, "calling gsutil to remove path")
	}

	return nil
}
