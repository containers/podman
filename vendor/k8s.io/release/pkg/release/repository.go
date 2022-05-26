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
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"sigs.k8s.io/release-sdk/git"
)

// Repo is a wrapper around a kubernetes/release repository
type Repo struct {
	repo Repository
}

// NewRepo creates a new release repository
func NewRepo() *Repo {
	return &Repo{}
}

// Repository is an interface for interacting with a git repository
//counterfeiter:generate . Repository
type Repository interface {
	Describe(opts *git.DescribeOptions) (string, error)
	CurrentBranch() (branch string, err error)
	RevParse(rev string) (string, error)
	Head() (string, error)
	Remotes() (res []*git.Remote, err error)
	LsRemote(...string) (string, error)
	IsDirty() (bool, error)
}

// Open assumes the current working directory as repository root and tries to
// open it
func (r *Repo) Open() error {
	dir, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "getting current working directory")
	}
	repo, err := git.OpenRepo(dir)
	if err != nil {
		return errors.Wrap(err, "opening release repository")
	}
	r.repo = repo
	return nil
}

// SetRepo can be used to set the internal repository implementation
func (r *Repo) SetRepo(repo Repository) {
	r.repo = repo
}

// GetTag returns the tag from the current repository
func (r *Repo) GetTag() (string, error) {
	describeOutput, err := r.repo.Describe(
		git.NewDescribeOptions().
			WithAlways().
			WithDirty().
			WithTags(),
	)
	if err != nil {
		return "", errors.Wrap(err, "running git describe")
	}
	t := time.Now().Format("20060102")
	return fmt.Sprintf("%s-%s", describeOutput, t), nil
}

// CheckState verifies that the repository is in the requested state
func (r *Repo) CheckState(expOrg, expRepo, expRev string, nomock bool) error {
	logrus.Info("Verifying repository state")

	dirty, err := r.repo.IsDirty()
	if err != nil {
		return errors.Wrap(err, "checking if repository is dirty")
	}
	if dirty {
		return errors.New(
			"repository is dirty, please commit and push your changes",
		)
	}
	logrus.Info("Repository is in clean state")

	branch, err := r.repo.CurrentBranch()
	if err != nil {
		return errors.Wrap(err, "retrieving current branch")
	}

	head, err := r.repo.Head()
	if err != nil {
		return errors.Wrap(err, "retrieving repository HEAD")
	}

	logrus.Infof("Repo head is: %s", head)

	rev, err := r.repo.RevParse(expRev)
	if err != nil {
		return errors.Wrapf(err, "retrieving rev-parse for %s", expRev)
	}

	if rev != head {
		return errors.Errorf("revision %q expected but got %q", head, rev)
	}

	if nomock && !(expOrg == DefaultToolOrg && expRepo == DefaultToolRepo && expRev == DefaultToolRef) {
		return errors.New("disallow using anything other than kubernetes/release:master with nomock flag")
	}

	logrus.Infof("Found matching revision %q", expRev)

	// Verify the remote
	remotes, err := r.repo.Remotes()
	if err != nil {
		return errors.Wrap(err, "retrieving repository remotes")
	}
	var foundRemote *git.Remote
	for _, remote := range remotes {
		for _, url := range remote.URLs() {
			if strings.Contains(url, filepath.Join(expOrg, expRepo)) {
				foundRemote = remote
				break
			}
		}
		if foundRemote != nil {
			break
		}
	}
	if foundRemote == nil {
		return errors.Errorf(
			"unable to find remote matching organization %q and repository %q",
			expOrg, expRepo,
		)
	}
	logrus.Infof(
		"Found matching organization %q and repository %q in remote: %s (%s)",
		expOrg,
		expRepo,
		foundRemote.Name(),
		strings.Join(foundRemote.URLs(), ", "),
	)

	logrus.Info("Verifying remote HEAD commit")
	args := []string{
		"--heads",
		foundRemote.Name(),
		fmt.Sprintf("refs/heads/%s", branch),
	}
	if branch == "" {
		args = []string{
			"--tags",
			"--heads",
			foundRemote.Name(),
			fmt.Sprintf("refs/tags/%s^{}", expRev),
		}
	}
	lsRemoteOut, err := r.repo.LsRemote(args...)
	if err != nil {
		return errors.Wrap(err, "getting remote HEAD")
	}
	fields := strings.Fields(lsRemoteOut)
	if len(fields) < 1 {
		return errors.Errorf("unexpected output: %s", lsRemoteOut)
	}
	commit := fields[0]
	logrus.Infof("Got remote commit: %s", commit)

	logrus.Info("Verifying that remote commit is equal to the local one")
	if head != commit {
		return errors.Errorf(
			"Local HEAD (%s) is not equal to latest remote commit (%s)",
			head, commit,
		)
	}
	logrus.Info("Repository is up-to-date")

	return nil
}
