/*
Copyright 2019 The Kubernetes Authors.

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

package options

import (
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"sigs.k8s.io/release-sdk/git"
	"sigs.k8s.io/release-sdk/github"
)

// Options is the global options structure which can be used to build release
// notes generator options
type Options struct {
	// GithubBaseURL specifies the Github base URL.
	GithubBaseURL string

	// GithubUploadURL specifies the Github upload URL.
	GithubUploadURL string

	// GithubOrg specifies the GitHub organization from which will be
	// cloned/pulled if Pull is true.
	GithubOrg string

	// GithubRepo specifies the GitHub repository from which will be
	// cloned/pulled if Pull is true.
	GithubRepo string

	// RepoPath specifies the git repository location for doing an update if
	// Pull is true.
	RepoPath string

	// Branch will be used for discovering the latest patch version if
	// DiscoverMode is RevisionDiscoveryModePatchToPatch.
	Branch string

	// StartSHA can be used to set the release notes start revision to an
	// exact git SHA. Should not be used together with StartRev.
	StartSHA string

	// EndSHA can be used to set the release notes end revision to an
	// exact git SHA. Should not be used together with EndRev.
	EndSHA string

	// StartRev can be used to set the release notes start revision to any
	// valid git revision. Should not be used together with StartSHA.
	StartRev string

	// EndRev can be used to set the release notes end revision to any
	// valid git revision. Should not be used together with EndSHA.
	EndRev string

	// Format specifies the format of the release notes. Can be either
	// `json` or `markdown`.
	Format string

	// If the `Format` is `markdown`, then this specifies the selected go
	// template. Can be `go-template:default`, `go-template:<file.template>` or
	// `go-template:inline:<template>`.
	GoTemplate string

	// RequiredAuthor can be used to filter the release notes by the commit
	// author
	RequiredAuthor string

	// DiscoverMode can be used to automatically discover StartSHA and EndSHA.
	// Can be either RevisionDiscoveryModeNONE (default),
	// RevisionDiscoveryModeMergeBaseToLatest,
	// RevisionDiscoveryModePatchToPatch, or RevisionDiscoveryModeMinorToMinor.
	// Should not be used together with StartRev, EndRev, StartSHA or EndSHA.
	DiscoverMode string

	// ReleaseTars specifies the directory where the release tarballs are
	// located.
	ReleaseTars string

	// ReleaseBucket specifies the Google Cloud bucket where the ReleaseTars
	// are linked to. This option is used for generating the links inside the
	// release downloads table.
	ReleaseBucket string

	// If true, then the release notes generator will pull in latest changes
	// from the default git remote
	Pull bool

	// If true, then the release notes generator will print messages in debug
	// log level
	Debug bool

	// EXPERIMENTAL: Feature flag for using v2 implementation to list commits
	ListReleaseNotesV2 bool

	// RecordDir specifies the directory for API call recordings. Cannot be
	// used together with ReplayDir.
	RecordDir string

	// ReplayDir specifies the directory for replaying a previously recorded
	// API. Cannot be used together with RecordDir.
	ReplayDir string

	githubToken string
	gitCloneFn  func(string, string, string, bool) (*git.Repo, error)

	// MapProviders list of release notes map providers to query during generations
	MapProviderStrings []string

	// If true, links for PRs and authors are added in the markdown format.
	// This is useful when the release notes are outputted to a file. When using the GitHub release page to publish release notes,
	// this option should be set to false to take advantage of Github's autolinked references.
	AddMarkdownLinks bool
}

type RevisionDiscoveryMode string

const (
	RevisionDiscoveryModeNONE              = "none"
	RevisionDiscoveryModeMergeBaseToLatest = "mergebase-to-latest"
	RevisionDiscoveryModePatchToPatch      = "patch-to-patch"
	RevisionDiscoveryModePatchToLatest     = "patch-to-latest"
	RevisionDiscoveryModeMinorToMinor      = "minor-to-minor"
)

const (
	FormatJSON     = "json"
	FormatMarkdown = "markdown"

	GoTemplatePrefix       = "go-template:"
	GoTemplatePrefixInline = "inline:"
	GoTemplateDefault      = GoTemplatePrefix + "default"
	GoTemplateInline       = GoTemplatePrefix + GoTemplatePrefixInline
)

// New creates a new Options instance with the default values
func New() *Options {
	return &Options{
		DiscoverMode:       RevisionDiscoveryModeNONE,
		GithubOrg:          git.DefaultGithubOrg,
		GithubRepo:         git.DefaultGithubRepo,
		Format:             FormatMarkdown,
		GoTemplate:         GoTemplateDefault,
		Pull:               true,
		gitCloneFn:         git.CloneOrOpenGitHubRepo,
		MapProviderStrings: []string{},
		AddMarkdownLinks:   false,
	}
}

// ValidateAndFinish checks if the options are set in a consistent way and
// adapts them if necessary. It returns an error if options are set to invalid
// values.
func (o *Options) ValidateAndFinish() (err error) {
	// Add appropriate log filtering
	if o.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if o.ReplayDir != "" && o.RecordDir != "" {
		return errors.New("please do not use record and replay together")
	}

	// Recover for replay if needed
	if o.ReplayDir != "" {
		logrus.Info("Using replay mode")
		return nil
	}

	// The GitHub Token is required if replay is not specified
	token, ok := os.LookupEnv(github.TokenEnvKey)
	if ok {
		o.githubToken = token
	} else if o.ReplayDir == "" {
		return errors.Errorf(
			"neither environment variable `%s` nor `replay` option is set",
			github.TokenEnvKey,
		)
	}

	// Check if we want to automatically discover the revisions
	if o.DiscoverMode != RevisionDiscoveryModeNONE {
		if err := o.resolveDiscoverMode(); err != nil {
			return err
		}
	}

	// The start SHA or rev is required.
	if o.StartSHA == "" && o.StartRev == "" {
		return errors.New("the starting commit hash must be set via --start-sha, $START_SHA, --start-rev or $START_REV")
	}

	// The end SHA or rev is required.
	if o.EndSHA == "" && o.EndRev == "" {
		return errors.New("the ending commit hash must be set via --end-sha, $END_SHA, --end-rev or $END_REV")
	}

	// Check if we have to parse a revision
	if (o.StartRev != "" && o.StartSHA == "") || (o.EndRev != "" && o.EndSHA == "") {
		repo, err := o.repo()
		if err != nil {
			return err
		}
		if o.StartRev != "" && o.StartSHA == "" {
			sha, err := repo.RevParseTag(o.StartRev)
			if err != nil {
				return errors.Wrapf(err, "resolving %s", o.StartRev)
			}
			logrus.Infof("Using found start SHA: %s", sha)
			o.StartSHA = sha
		}
		if o.EndRev != "" && o.EndSHA == "" {
			sha, err := repo.RevParseTag(o.EndRev)
			if err != nil {
				return errors.Wrapf(err, "resolving %s", o.EndRev)
			}
			logrus.Infof("Using found end SHA: %s", sha)
			o.EndSHA = sha
		}
	}

	// Create the record dir
	if o.RecordDir != "" {
		logrus.Info("Using record mode")
		if err := os.MkdirAll(o.RecordDir, os.FileMode(0o755)); err != nil {
			return err
		}
	}

	// Set GithubBaseURL to https://github.com if it is unset.
	if o.GithubBaseURL == "" {
		o.GithubBaseURL = github.GitHubURL
	}

	if err := o.checkFormatOptions(); err != nil {
		return errors.Wrap(err, "while checking format flags")
	}
	return nil
}

// checkFormatOptions verifies that template related options are sane
func (o *Options) checkFormatOptions() error {
	// Validate the output format and template
	logrus.Infof("Using output format: %s", o.Format)
	if o.Format == FormatMarkdown && o.GoTemplate != GoTemplateDefault {
		if !strings.HasPrefix(o.GoTemplate, GoTemplatePrefix) {
			return errors.Errorf("go template has to be prefixed with %q", GoTemplatePrefix)
		}

		templatePathOrOnline := strings.TrimPrefix(o.GoTemplate, GoTemplatePrefix)
		// Verify if template file exists
		if !strings.HasPrefix(templatePathOrOnline, GoTemplatePrefixInline) {
			fileStats, err := os.Stat(templatePathOrOnline)
			if os.IsNotExist(err) {
				return errors.Errorf("could not find template file (%s)", templatePathOrOnline)
			}
			if fileStats.Size() == 0 {
				return errors.Errorf("template file %s is empty", templatePathOrOnline)
			}
		}
	}
	if o.Format == FormatJSON && o.GoTemplate != GoTemplateDefault {
		return errors.New("go-template cannot be defined when in JSON mode")
	}
	if o.Format != FormatJSON && o.Format != FormatMarkdown {
		return errors.Errorf("invalid format: %s", o.Format)
	}
	return nil
}

func (o *Options) resolveDiscoverMode() error {
	repo, err := o.repo()
	if err != nil {
		return err
	}

	var result git.DiscoverResult
	if o.DiscoverMode == RevisionDiscoveryModeMergeBaseToLatest {
		result, err = repo.LatestReleaseBranchMergeBaseToLatest()
	} else if o.DiscoverMode == RevisionDiscoveryModePatchToPatch {
		result, err = repo.LatestPatchToPatch(o.Branch)
	} else if o.DiscoverMode == RevisionDiscoveryModePatchToLatest {
		result, err = repo.LatestPatchToLatest(o.Branch)
	} else if o.DiscoverMode == RevisionDiscoveryModeMinorToMinor {
		result, err = repo.LatestNonPatchFinalToMinor()
	}
	if err != nil {
		return err
	}

	o.StartSHA = result.StartSHA()
	o.StartRev = result.StartRev()
	o.EndSHA = result.EndSHA()
	o.EndRev = result.EndRev()

	logrus.Infof("Discovered start SHA %s", o.StartSHA)
	logrus.Infof("Discovered end SHA %s", o.EndSHA)

	logrus.Infof("Using start revision %s", o.StartRev)
	logrus.Infof("Using end revision %s", o.EndRev)

	return nil
}

func (o *Options) repo() (repo *git.Repo, err error) {
	if o.Pull {
		logrus.Infof("Cloning/updating repository %s/%s", o.GithubOrg, o.GithubRepo)
		repo, err = o.gitCloneFn(
			o.RepoPath,
			o.GithubOrg,
			o.GithubRepo,
			false,
		)
	} else {
		logrus.Infof("Re-using local repo %s", o.RepoPath)
		repo, err = git.OpenRepo(o.RepoPath)
	}
	if err != nil {
		return nil, err
	}
	return repo, nil
}

// Client returns a Client to be used by the Gatherer. Depending on
// the provided options this is either a real client talking to the GitHub API,
// a Client which in addition records the responses from Github and stores them
// on disk, or a Client that replays those pre-recorded responses and does not
// talk to the GitHub API at all.
func (o *Options) Client() (github.Client, error) {
	if o.ReplayDir != "" {
		return github.NewReplayer(o.ReplayDir), nil
	}

	var gh *github.GitHub
	var err error
	// Create a real GitHub API client
	if o.GithubBaseURL != "" && o.GithubUploadURL != "" {
		gh, err = github.NewEnterpriseWithToken(o.GithubBaseURL, o.GithubUploadURL, o.githubToken)
	} else {
		gh, err = github.NewWithToken(o.githubToken)
	}
	if err != nil {
		return nil, errors.Wrap(err, "unable to create GitHub client")
	}

	if o.RecordDir != "" {
		return github.NewRecorder(gh.Client(), o.RecordDir), nil
	}

	return gh.Client(), nil
}
