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

package github

import (
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"sigs.k8s.io/release-sdk/git"
)

const (
	// UserForkName is the name we will give to the user's remote when adding
	// it to repos
	UserForkName = "userfork"
)

// PrepareFork prepares a branch from a repo fork
func PrepareFork(branchName, upstreamOrg, upstreamRepo, myOrg, myRepo string) (repo *git.Repo, err error) {
	// checkout the upstream repository
	logrus.Infof("Cloning/updating repository %s/%s", upstreamOrg, upstreamRepo)

	repo, err = git.CleanCloneGitHubRepo(
		upstreamOrg, upstreamRepo, false,
	)
	if err != nil {
		return nil, errors.Wrapf(err, "cloning %s/%s", upstreamOrg, upstreamRepo)
	}

	// test if the fork remote is already existing
	url := git.GetRepoURL(myOrg, myRepo, false)
	if repo.HasRemote(UserForkName, url) {
		logrus.Infof(
			"Using already existing remote %s (%s) in repository",
			UserForkName, url,
		)
	} else {
		// add the user's fork as a remote
		err = repo.AddRemote(UserForkName, myOrg, myRepo)
		if err != nil {
			return nil, errors.Wrap(err, "adding user's fork as remote repository")
		}
	}

	// checkout the new branch
	err = repo.Checkout("-B", branchName)
	if err != nil {
		return nil, errors.Wrapf(err, "creating new branch %s", branchName)
	}

	return repo, nil
}

// VerifyFork does a pre-check of a fork to see if we can create a PR from it
func VerifyFork(branchName, forkOwner, forkRepo, parentOwner, parentRepo string) error {
	logrus.Infof("Checking if a PR can be created from %s/%s", forkOwner, forkRepo)
	gh := New()

	// check if the specified repo is a fork of the parent
	isRepo, err := gh.RepoIsForkOf(
		forkOwner, forkRepo, parentOwner, parentRepo,
	)
	if err != nil {
		return errors.Wrapf(
			err, "while checking if repository is a fork of %s/%s",
			parentOwner, parentRepo,
		)
	}

	if !isRepo {
		return errors.Errorf(
			"cannot create PR, %s/%s is not a fork of %s/%s",
			forkOwner, forkRepo, parentOwner, parentRepo,
		)
	}

	// verify the branch does not exist
	branchExists, err := gh.BranchExists(
		forkOwner, forkRepo, branchName,
	)
	if err != nil {
		return errors.Wrap(err, "while checking if branch can be created")
	}

	if branchExists {
		return errors.Errorf(
			"a branch named %s already exists in %s/%s",
			branchName, forkOwner, forkRepo,
		)
	}
	return nil
}
