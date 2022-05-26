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

package notes

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"k8s.io/release/pkg/notes/options"

	"github.com/cheggaaa/pb/v3"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	gitobject "github.com/go-git/go-git/v5/plumbing/object"
	"github.com/mattn/go-isatty"
	"github.com/nozzle/throttler"
	"github.com/sirupsen/logrus"
)

type commitPrPair struct {
	Commit *gitobject.Commit
	PrNum  int
}

type releaseNotesAggregator struct {
	releaseNotes *ReleaseNotes
	sync.RWMutex
}

func (g *Gatherer) ListReleaseNotesV2() (*ReleaseNotes, error) {
	// left parent of Git commits is always the main branch parent
	pairs, err := g.listLeftParentCommits(g.options)
	if err != nil {
		return nil, errors.Wrap(err, "listing offline commits")
	}

	// load map providers specified in options
	mapProviders := []MapProvider{}
	for _, initString := range g.options.MapProviderStrings {
		provider, err := NewProviderFromInitString(initString)
		if err != nil {
			return nil, errors.Wrap(err, "while getting release notes map providers")
		}
		mapProviders = append(mapProviders, provider)
	}

	t := throttler.New(maxParallelRequests, len(pairs))

	aggregator := releaseNotesAggregator{
		releaseNotes: NewReleaseNotes(),
	}

	pairsCount := len(pairs)
	logrus.Infof("processing release notes for %d commits", pairsCount)

	// display progress bar in stdout, since stderr is used by logger
	bar := pb.New(pairsCount).SetWriter(os.Stdout)

	// only display progress bar in user TTY
	if isatty.IsTerminal(os.Stdout.Fd()) {
		bar.Start()
	}

	for _, pair := range pairs {
		// pair needs to be scoped in parameter so that the specific variable read
		// happens when the goroutine is declared, not when referenced inside
		go func(pair *commitPrPair) {
			noteMaps := []*ReleaseNotesMap{}
			for _, provider := range mapProviders {
				noteMaps, err = provider.GetMapsForPR(pair.PrNum)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"pr": pair.PrNum,
					}).Errorf("ignore err: %v", err)
					noteMaps = []*ReleaseNotesMap{}
				}
			}

			releaseNote, err := g.buildReleaseNote(pair)
			if err == nil {
				if releaseNote != nil {
					for _, noteMap := range noteMaps {
						if err := releaseNote.ApplyMap(noteMap, g.options.AddMarkdownLinks); err != nil {
							logrus.WithFields(logrus.Fields{
								"pr": pair.PrNum,
							}).Errorf("ignore err: %v", err)
						}
					}
					logrus.WithFields(logrus.Fields{
						"pr":   pair.PrNum,
						"note": releaseNote.Text,
					}).Debugf("finalized release note")
					aggregator.Lock()
					aggregator.releaseNotes.Set(pair.PrNum, releaseNote)
					aggregator.Unlock()
				} else {
					logrus.WithFields(logrus.Fields{
						"pr": pair.PrNum,
					}).Debugf("skip: empty release note")
				}
			} else {
				logrus.WithFields(logrus.Fields{
					"sha": pair.Commit.Hash.String(),
					"pr":  pair.PrNum,
				}).Errorf("err: %v", err)
			}
			bar.Increment()
			t.Done(nil)
		}(pair)

		if t.Throttle() > 0 {
			break
		}
	}

	if err := t.Err(); err != nil {
		return nil, err
	}

	bar.Finish()

	return aggregator.releaseNotes, nil
}

func (g *Gatherer) buildReleaseNote(pair *commitPrPair) (*ReleaseNote, error) {
	pr, _, err := g.client.GetPullRequest(g.context, g.options.GithubOrg, g.options.GithubRepo, pair.PrNum)
	if err != nil {
		return nil, err
	}

	prBody := pr.GetBody()

	if MatchesExcludeFilter(prBody) {
		return nil, nil
	}

	text, err := noteTextFromString(prBody)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"sha": pair.Commit.Hash.String(),
			"pr":  pair.PrNum,
		}).Debugf("ignore err: %v", err)
		return nil, nil
	}

	documentation := DocumentationFromString(prBody)

	author := pr.GetUser().GetLogin()
	authorURL := pr.GetUser().GetHTMLURL()
	prURL := pr.GetHTMLURL()
	isFeature := hasString(labelsWithPrefix(pr, "kind"), "feature")
	noteSuffix := prettifySIGList(labelsWithPrefix(pr, "sig"))

	isDuplicateSIG := false
	if len(labelsWithPrefix(pr, "sig")) > 1 {
		isDuplicateSIG = true
	}

	isDuplicateKind := false
	if len(labelsWithPrefix(pr, "kind")) > 1 {
		isDuplicateKind = true
	}

	// TODO(wilsonehusin): extract / follow original in ReleasenoteFromCommit
	indented := strings.ReplaceAll(text, "\n", "\n  ")
	markdown := fmt.Sprintf("%s (#%d, @%s)",
		indented, pr.GetNumber(), author)
	if g.options.AddMarkdownLinks {
		markdown = fmt.Sprintf("%s ([#%d](%s), [@%s](%s))",
			indented, pr.GetNumber(), prURL, author, authorURL)
	}

	if noteSuffix != "" {
		markdown = fmt.Sprintf("%s [%s]", markdown, noteSuffix)
	}

	// Uppercase the first character of the markdown to make it look uniform
	markdown = capitalizeString(markdown)

	return &ReleaseNote{
		Commit:         pair.Commit.Hash.String(),
		Text:           text,
		Markdown:       markdown,
		Documentation:  documentation,
		Author:         author,
		AuthorURL:      authorURL,
		PrURL:          prURL,
		PrNumber:       pr.GetNumber(),
		SIGs:           labelsWithPrefix(pr, "sig"),
		Kinds:          labelsWithPrefix(pr, "kind"),
		Areas:          labelsWithPrefix(pr, "area"),
		Feature:        isFeature,
		Duplicate:      isDuplicateSIG,
		DuplicateKind:  isDuplicateKind,
		ActionRequired: labelExactMatch(pr, "release-note-action-required"),
		DoNotPublish:   labelExactMatch(pr, "release-note-none"),
	}, nil
}

func (g *Gatherer) listLeftParentCommits(opts *options.Options) ([]*commitPrPair, error) {
	localRepository, err := git.PlainOpen(opts.RepoPath)
	if err != nil {
		return nil, err
	}

	// opts.StartSHA points to a tag (e.g. 1.20.0) which is on a release branch (e.g. release-1.20)
	// this means traveling through commit history from opts.EndSHA will never reach opts.StartSHA

	// the stopping point to be set should be the last shared commit between release branch and primary (master) branch
	// usually, following the left / first parents, it would be

	// ^ master
	// |
	// * tag: 1.21.0-alpha.x / 1.21.0-beta.y
	// |
	// : :
	// | |
	// | * tag: v1.20.0, some merge commit pointed by opts.StartSHA
	// | |
	// | * Anago GCB release commit (begin branch out of release-1.20)
	// |/
	// x last shared commit

	// merge base would resolve to last shared commit, marked by (x)

	endCommit, err := localRepository.CommitObject(plumbing.NewHash(opts.EndSHA))
	if err != nil {
		return nil, errors.Wrap(err, "finding commit of EndSHA")
	}

	startCommit, err := localRepository.CommitObject(plumbing.NewHash(opts.StartSHA))
	if err != nil {
		return nil, errors.Wrap(err, "finding commit of StartSHA")
	}

	logrus.Debugf("finding merge base (last shared commit) between the two SHAs")
	startTime := time.Now()
	lastSharedCommits, err := endCommit.MergeBase(startCommit)
	if err != nil {
		return nil, errors.Wrap(err, "finding shared commits")
	}
	if len(lastSharedCommits) == 0 {
		return nil, fmt.Errorf("no shared commits between the provided SHAs")
	}
	logrus.Debugf("found merge base in %v", time.Since(startTime))

	stopHash := lastSharedCommits[0].Hash
	logrus.Infof("will stop at %s", stopHash.String())

	currentTagHash := plumbing.NewHash(opts.EndSHA)

	pairs := []*commitPrPair{}
	hashPointer := currentTagHash
	for hashPointer != stopHash {
		hashString := hashPointer.String()

		// Find and collect commit objects
		commitPointer, err := localRepository.CommitObject(hashPointer)
		if err != nil {
			return nil, errors.Wrap(err, "finding CommitObject")
		}

		// Find and collect PR number from commit message
		prNums, err := prsNumForCommitFromMessage(commitPointer.Message)
		if err == errNoPRIDFoundInCommitMessage {
			logrus.WithFields(logrus.Fields{
				"sha": hashString,
			}).Debug("no associated PR found")

			// Advance pointer based on left parent
			hashPointer = commitPointer.ParentHashes[0]
			continue
		}
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"sha": hashString,
			}).Warnf("ignore err: %v", err)

			// Advance pointer based on left parent
			hashPointer = commitPointer.ParentHashes[0]
			continue
		}
		logrus.WithFields(logrus.Fields{
			"sha": hashString,
			"prs": prNums,
		}).Debug("found PR from commit")

		// Only taking the first one, assuming they are merged by Prow
		pairs = append(pairs, &commitPrPair{Commit: commitPointer, PrNum: prNums[0]})

		// Advance pointer based on left parent
		hashPointer = commitPointer.ParentHashes[0]
	}

	return pairs, nil
}
