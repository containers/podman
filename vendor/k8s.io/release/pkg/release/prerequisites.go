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
	"strings"

	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/sirupsen/logrus"

	"sigs.k8s.io/release-sdk/gcli"
	"sigs.k8s.io/release-sdk/git"
	"sigs.k8s.io/release-sdk/github"
	"sigs.k8s.io/release-utils/command"
	"sigs.k8s.io/release-utils/env"
)

// PrerequisitesChecker is the main type for checking the prerequisites for a
// release.
type PrerequisitesChecker struct {
	impl prerequisitesCheckerImpl
	opts *PrerequisitesCheckerOptions
}

// Type prerequisites checker
type PrerequisitesCheckerOptions struct {
	CheckGitHubToken bool
}

var DefaultPrerequisitesCheckerOptions = &PrerequisitesCheckerOptions{
	CheckGitHubToken: true,
}

// NewPrerequisitesChecker creates a new PrerequisitesChecker instance.
func NewPrerequisitesChecker() *PrerequisitesChecker {
	return &PrerequisitesChecker{
		&defaultPrerequisitesChecker{},
		DefaultPrerequisitesCheckerOptions,
	}
}

// Options return the options from the prereq checker
func (p *PrerequisitesChecker) Options() *PrerequisitesCheckerOptions {
	return p.opts
}

// SetImpl can be used to set the internal PrerequisitesChecker implementation.
func (p *PrerequisitesChecker) SetImpl(impl prerequisitesCheckerImpl) {
	p.impl = impl
}

//counterfeiter:generate . prerequisitesCheckerImpl
type prerequisitesCheckerImpl interface {
	CommandAvailable(commands ...string) bool
	DockerVersion() (string, error)
	GCloudOutput(args ...string) (string, error)
	IsEnvSet(key string) bool
	Usage(dir string) (*disk.UsageStat, error)
	ConfigureGlobalDefaultUserAndEmail() error
}

type defaultPrerequisitesChecker struct{}

func (*defaultPrerequisitesChecker) CommandAvailable(
	commands ...string,
) bool {
	return command.Available(commands...)
}

func (*defaultPrerequisitesChecker) GCloudOutput(
	args ...string,
) (string, error) {
	return gcli.GCloudOutput(args...)
}

func (*defaultPrerequisitesChecker) IsEnvSet(key string) bool {
	return env.IsSet(key)
}

func (*defaultPrerequisitesChecker) ConfigureGlobalDefaultUserAndEmail() error {
	return git.ConfigureGlobalDefaultUserAndEmail()
}

func (*defaultPrerequisitesChecker) DockerVersion() (string, error) {
	res, err := command.New(
		"docker", "version", "--format", "{{.Client.Version}}",
	).Pipe("cut", "-d-", "-f1").RunSilentSuccessOutput()
	if err != nil {
		return "", err
	}
	return res.OutputTrimNL(), err
}

func (*defaultPrerequisitesChecker) Usage(dir string) (*disk.UsageStat, error) {
	return disk.Usage(dir)
}

func (p *PrerequisitesChecker) Run(workdir string) error {
	// Command checks
	commands := []string{
		"docker", "jq", "gsutil", "gcloud", "ssh",
	}
	logrus.Infof(
		"Verifying that the commands %s exist in $PATH.",
		strings.Join(commands, ", "),
	)

	if !p.impl.CommandAvailable(commands...) {
		return errors.Errorf("not all commands available")
	}

	// Docker version checks
	const minVersion = "18.06.0"
	logrus.Infof("Verifying minimum Docker version %s", minVersion)
	versionOutput, err := p.impl.DockerVersion()
	if err != nil {
		return errors.Wrap(err, "validate docker version")
	}
	if versionOutput < minVersion {
		return errors.Errorf(
			"minimum docker version %s required, got %s",
			minVersion, versionOutput,
		)
	}

	// Google Cloud checks
	logrus.Info("Verifying Google Cloud access")
	if _, err := p.impl.GCloudOutput(
		"config", "get-value", "project",
	); err != nil {
		return errors.Wrap(err, "no account authorized through gcloud")
	}

	// GitHub checks
	if p.opts.CheckGitHubToken {
		logrus.Infof(
			"Verifying that %s environemt variable is set", github.TokenEnvKey,
		)
		if !p.impl.IsEnvSet(github.TokenEnvKey) {
			return errors.Errorf("no %s env variable set", github.TokenEnvKey)
		}
	}

	// Disk space check
	const minDiskSpaceGiB = 100
	logrus.Infof(
		"Checking available disk space (%dGB) for %s", minDiskSpaceGiB, workdir,
	)
	res, err := p.impl.Usage(workdir)
	if err != nil {
		return errors.Wrap(err, "check available disk space")
	}
	diskSpaceGiB := res.Free / 1024 / 1024 / 1024
	if diskSpaceGiB < minDiskSpaceGiB {
		return errors.Errorf(
			"not enough disk space available. Got %dGiB, need at least %dGiB",
			diskSpaceGiB, minDiskSpaceGiB,
		)
	}

	// Git setup check
	logrus.Info("Configuring git user and email")
	if err := p.impl.ConfigureGlobalDefaultUserAndEmail(); err != nil {
		return errors.Wrap(err, "configure git user and email")
	}

	return nil
}
