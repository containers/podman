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
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"sigs.k8s.io/release-sdk/gcli"
	"sigs.k8s.io/release-sdk/object"
	"sigs.k8s.io/release-utils/command"
	"sigs.k8s.io/release-utils/tar"

	"sigs.k8s.io/release-utils/util"
)

const (
	archiveDirPrefix   = "anago-"  // Prefix for archive directories
	archiveBucketPath  = "archive" // Archiv sibdirectory in bucket
	logsArchiveSubPath = "logs"    // Logs subdirectory
)

// Archiver stores the release build directory in a bucket
// along with it's logs
type Archiver struct {
	impl archiverImpl
	opts *ArchiverOptions
}

// NewArchiver create a new archiver with the default implementation
func NewArchiver(opts *ArchiverOptions) *Archiver {
	return &Archiver{&defaultArchiverImpl{}, opts}
}

// SetImpl changes the archiver implementation
func (archiver *Archiver) SetImpl(impl archiverImpl) {
	archiver.impl = impl
}

// ArchiverOptions set the options used when archiving a release
type ArchiverOptions struct {
	ReleaseBuildDir string // Build directory that will be archived
	LogFile         string // Log file to process and include in the archive
	PrimeVersion    string // Final version tag
	BuildVersion    string // Build version from where this release has cut
	Bucket          string // Bucket we will use to archive and read staged data
}

// ArchiveBucketPath returns the bucket path we the release will be stored
func (o *ArchiverOptions) ArchiveBucketPath() string {
	// local archive_bucket="gs://$RELEASE_BUCKET/archive"
	if o.Bucket == "" || o.PrimeVersion == "" {
		return ""
	}
	gcs := object.NewGCS()
	archiveBucketPath, err := gcs.NormalizePath(
		object.GcsPrefix + filepath.Join(o.Bucket, ArchivePath, archiveDirPrefix+o.PrimeVersion),
	)
	if err != nil {
		logrus.Error(err)
		return ""
	}
	return archiveBucketPath
}

// Validate checks if the set values are correct and complete to
// start running the archival process
func (o *ArchiverOptions) Validate() error {
	if o.LogFile == "" {
		return errors.New("release log file was not specified")
	}
	if !util.Exists(o.ReleaseBuildDir) {
		return errors.New("GCB worskapce directory does not exist")
	}
	if !util.Exists(o.LogFile) {
		return errors.New("logs file not found")
	}
	if o.BuildVersion == "" {
		return errors.New("build version tag in archiver options is empty")
	}
	if o.PrimeVersion == "" {
		return errors.New("prime version tag in archiver options is empty")
	}
	if o.Bucket == "" {
		return errors.New("archive bucket is not specified")
	}

	// Check if the build version is well formed (used for cleaning old staged build)
	if _, err := util.TagStringToSemver(o.BuildVersion); err != nil {
		return errors.Wrap(err, "verifying build version tag")
	}

	// Check if the prime version is well formed
	if _, err := util.TagStringToSemver(o.PrimeVersion); err != nil {
		return errors.Wrap(err, "verifying prime version tag")
	}

	return nil
}

//counterfeiter:generate . archiverImpl
type archiverImpl interface {
	CopyReleaseToBucket(string, string) error
	DeleteStalePasswordFiles(string) error
	MakeFilesPrivate(string) error
	ValidateOptions(*ArchiverOptions) error
	CopyReleaseLogs([]string, string, string) error
	CleanStagedBuilds(string, string) error
}

type defaultArchiverImpl struct{}

// ArchiveRelease stores the release directory and logs in a GCP
// bucket for archival purposes. Log files are sanitized and made private
func (archiver *Archiver) ArchiveRelease() error {
	// Verify options are complete
	if err := archiver.impl.ValidateOptions(archiver.opts); err != nil {
		return errors.Wrap(err, "validating archive options")
	}

	// TODO: Is this still relevant?
	// local text="files"

	// # TODO: Copy $PROGSTATE as well to GCS and restore it if found
	// # also delete if complete or just delete once copied back to $TMPDIR
	// # This is so failures on GCB can be restarted / reentrant too.

	// if [[ $arg != "--files-only" ]]; then
	//  dash_args="-rc"
	//   text="contents"
	// fi

	// Remove temporary password file so not to alarm passers-by.
	if err := archiver.impl.DeleteStalePasswordFiles(
		archiver.opts.ReleaseBuildDir,
	); err != nil {
		return errors.Wrap(err, "looking for stale password files")
	}

	// Clean previous staged builds
	if err := archiver.impl.CleanStagedBuilds(
		object.GcsPrefix+filepath.Join(archiver.opts.Bucket, StagePath),
		archiver.opts.BuildVersion,
	); err != nil {
		return errors.Wrap(err, "deleting previous staged builds")
	}

	// Copy the release to the bucket
	if err := archiver.impl.CopyReleaseToBucket(
		archiver.opts.ReleaseBuildDir,
		archiver.opts.ArchiveBucketPath(),
	); err != nil {
		return errors.Wrap(err, "while copying the release directory")
	}

	// copy_logs_to_workdir
	if err := archiver.impl.CopyReleaseLogs(
		[]string{archiver.opts.LogFile},
		filepath.Join(archiver.opts.ReleaseBuildDir, logsArchiveSubPath),
		filepath.Join(archiver.opts.ArchiveBucketPath(), logsArchiveSubPath),
	); err != nil {
		return errors.Wrap(err, "copying release logs to archive")
	}

	// Make the logs private (remove AllUsers from the GCS ACL)
	if err := archiver.impl.MakeFilesPrivate(
		filepath.Join(archiver.opts.ArchiveBucketPath(), logsArchiveSubPath),
	); err != nil {
		return errors.Wrapf(err, "setting private ACL on logs")
	}

	logrus.Info("Release archive complete")
	return nil
}

// validateOptions runs the options validation
func (a *defaultArchiverImpl) ValidateOptions(o *ArchiverOptions) error {
	return errors.Wrap(o.Validate(), "validating options")
}

// makeFilesPrivate updates the ACL on all files in a directory
func (a *defaultArchiverImpl) MakeFilesPrivate(archiveBucketPath string) error {
	logrus.Infof("Ensure PRIVATE ACL on %s/*", archiveBucketPath)
	gcs := object.NewGCS()
	logsPath, err := gcs.NormalizePath(archiveBucketPath + "/*")
	if err != nil {
		return errors.Wrap(err, "normalizing gcs path to modify ACL")
	}
	// logrun -s $GSUTIL acl ch -d AllUsers "$archive_bucket/$build_dir/${LOGFILE##*/}*" || true
	if err := gcli.GSUtil("acl", "ch", "-d", "AllUsers", logsPath); err != nil {
		return errors.Wrapf(err, "removing public access from files in %s", archiveBucketPath)
	}
	return nil
}

// deleteStalePasswordFiles emoves temporary password file so not to alarm passers-by.
func (a *defaultArchiverImpl) DeleteStalePasswordFiles(releaseBuildDir string) error {
	if err := command.NewWithWorkDir(
		releaseBuildDir, "find", "-type", "f", "-name", "rsyncd.password", "-delete",
	).RunSuccess(); err != nil {
		return errors.Wrap(err, "deleting temporary password files")
	}

	// Delete the git remote config to avoid it ending in the stage bucket
	gitConf := filepath.Join(releaseBuildDir, "k8s.io/kubernetes/.git/config")
	if util.Exists(gitConf) {
		if err := os.Remove(gitConf); err != nil {
			return errors.Wrap(err, "deleting git remote config")
		}
	} else {
		logrus.Warn("git configuration file not found, nothing to remove")
	}

	return nil
}

// copyReleaseLogs gets a slice of log file names. Those files are
// sanitized to remove sensitive data and control characters and then are
// copied to the GCB working directory.
func (a *defaultArchiverImpl) CopyReleaseLogs(
	logFiles []string, targetDir, archiveBucketLogsPath string,
) (err error) {
	// Verify the destination bucket address is correct
	gcs := object.NewGCS()
	if archiveBucketLogsPath != "" {
		archiveBucketLogsPath, err = gcs.NormalizePath(archiveBucketLogsPath)
		if err != nil {
			return errors.Wrap(err, "normalizing remote logfile destination")
		}
	}
	// Check the destination directory exists
	if !util.Exists(targetDir) {
		if err := os.Mkdir(targetDir, os.FileMode(0o755)); err != nil {
			return errors.Wrap(err, "creating logs archive directory")
		}
	}
	for _, fileName := range logFiles {
		// Strip the logfiles from control chars and sensitive data
		if err := util.CleanLogFile(fileName); err != nil {
			return errors.Wrap(err, "sanitizing logfile")
		}

		logrus.Infof("Copying %s to %s", fileName, targetDir)
		if err := util.CopyFileLocal(
			fileName, filepath.Join(targetDir, filepath.Base(fileName)), true,
		); err != nil {
			return errors.Wrapf(err, "Copying logfile %s to %s", fileName, targetDir)
		}
	}
	// TODO: Grab previous log files from stage and copy them to logs dir

	// Rsync log files to remote location if a bucket is specified
	if archiveBucketLogsPath != "" {
		logrus.Infof("Rsyncing logs to remote bucket %s", archiveBucketLogsPath)
		if err := gcs.RsyncRecursive(targetDir, archiveBucketLogsPath); err != nil {
			return errors.Wrap(err, "while synching log files to remote bucket addr")
		}
	}
	return nil
}

// copyReleaseToBucket Copies the release directory to the specified bucket location
func (a *defaultArchiverImpl) CopyReleaseToBucket(releaseBuildDir, archiveBucketPath string) error {
	// TODO: Check if we have write access to the bucket?

	// Create a GCS cliente to copy the release
	gcs := object.NewGCS()
	remoteDest, err := gcs.NormalizePath(archiveBucketPath)
	if err != nil {
		return errors.Wrap(err, "normalizing destination path")
	}

	srcPath := filepath.Join(releaseBuildDir, "k8s.io")
	tarball := srcPath + ".tar.gz"
	logrus.Infof("Compressing %s to %s", srcPath, tarball)
	if err := tar.Compress(tarball, srcPath); err != nil {
		return errors.Wrap(err, "create source tarball")
	}

	logrus.Infof("Removing source path %s before syncing", srcPath)
	if err := os.RemoveAll(srcPath); err != nil {
		return errors.Wrap(err, "remove source path")
	}

	logrus.Infof("Rsync %s to %s", releaseBuildDir, remoteDest)
	if err := gcs.RsyncRecursive(releaseBuildDir, remoteDest); err != nil {
		return errors.Wrap(err, "copying release directory to bucket")
	}
	return nil
}

// GetLogFiles reads a directory and returns the files that are anago logs
func (a *defaultArchiverImpl) GetLogFiles(logsDir string) ([]string, error) {
	logFiles := []string{}
	tmpContents, err := os.ReadDir(logsDir)
	if err != nil {
		return nil, errors.Wrapf(err, "searching for logfiles in %s", logsDir)
	}
	for _, finfo := range tmpContents {
		if strings.HasPrefix(finfo.Name(), "anago") &&
			strings.Contains(finfo.Name(), ".log") {
			logFiles = append(logFiles, filepath.Join(logsDir, finfo.Name()))
		}
	}
	return logFiles, nil
}

// CleanStagedBuilds removes all past staged builds from the same
// Major.Minor version we are running now
func (a *defaultArchiverImpl) CleanStagedBuilds(bucketPath, buildVersion string) error {
	// Build the prefix we will be looking for
	semver, err := util.TagStringToSemver(buildVersion)
	if err != nil {
		return errors.Wrap(err, "parsing semver from tag")
	}
	dirPrefix := fmt.Sprintf("%s%d.%d", util.TagPrefix, semver.Major, semver.Minor)

	// Normalize the bucket parh
	// Build a GCS object to delete old builds
	gcs := object.NewGCS()
	gcs.SetOptions(
		gcs.WithConcurrent(true),
		gcs.WithRecursive(true),
	)

	// Normalize the bucket path
	path, err := gcs.NormalizePath(bucketPath, dirPrefix+"*")
	if err != nil {
		return errors.Wrap(err, "normalizing stage path")
	}

	// Get all staged build that match the pattern
	output, err := gcli.GSUtilOutput("ls", "-d", path)
	if err != nil {
		return errors.Wrap(err, "listing bucket contents")
	}

	for _, line := range strings.Fields(output) {
		if strings.Contains(line, dirPrefix) && !strings.Contains(line, buildVersion) {
			logrus.Infof("Deleting previous staged build: %s", line)
			if err := gcs.DeletePath(line); err != nil {
				return errors.Wrap(err, "calling gsutil to delete build")
			}
		}
	}
	return nil
}
