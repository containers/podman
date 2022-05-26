/*
Copyright 2017 The Kubernetes Authors.

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
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // used for file integrity checks, NOT security
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	gogithub "github.com/google/go-github/v39/github"
	"github.com/nozzle/throttler"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"k8s.io/release/pkg/notes/options"
	"sigs.k8s.io/release-sdk/github"
)

var (
	errNoPRIDFoundInCommitMessage       = errors.New("no PR IDs found in the commit message")
	errNoPRFoundForCommitSHA            = errors.New("no PR found for this commit")
	apiSleepTime                  int64 = 60
)

const (
	DefaultOrg  = "kubernetes"
	DefaultRepo = "kubernetes"

	// maxParallelRequests is the maximum parallel requests we shall make to the
	// GitHub API
	maxParallelRequests = 10
)

type (
	Notes []string
	Kind  string
)

const (
	KindAPIChange     Kind = "api-change"
	KindBug           Kind = "bug"
	KindCleanup       Kind = "cleanup"
	KindDeprecation   Kind = "deprecation"
	KindDesign        Kind = "design"
	KindDocumentation Kind = "documentation"
	KindFailingTest   Kind = "failing-test"
	KindFeature       Kind = "feature"
	KindFlake         Kind = "flake"
	KindRegression    Kind = "regression"
	KindOther         Kind = "Other (Cleanup or Flake)"
	KindUncategorized Kind = "Uncategorized"
)

// ReleaseNote is the type that represents the total sum of all the information
// we've gathered about a single release note.
type ReleaseNote struct {
	// Commit is the SHA of the commit which is the source of this note. This is
	// also effectively a unique ID for release notes.
	Commit string `json:"commit"`

	// Text is the actual content of the release note
	Text string `json:"text"`

	// Markdown is the markdown formatted note
	Markdown string `json:"markdown"`

	// Docs is additional documentation for the release note
	Documentation []*Documentation `json:"documentation,omitempty"`

	// Author is the GitHub username of the commit author
	Author string `json:"author"`

	// AuthorURL is the GitHub URL of the commit author
	AuthorURL string `json:"author_url"`

	// PrURL is a URL to the PR
	PrURL string `json:"pr_url"`

	// PrNumber is the number of the PR
	PrNumber int `json:"pr_number"`

	// Areas is a list of the labels beginning with area/
	Areas []string `json:"areas,omitempty"`

	// Kinds is a list of the labels beginning with kind/
	Kinds []string `json:"kinds,omitempty"`

	// SIGs is a list of the labels beginning with sig/
	SIGs []string `json:"sigs,omitempty"`

	// Indicates whether or not a note will appear as a new feature
	Feature bool `json:"feature,omitempty"`

	// Indicates whether or not a note is duplicated across SIGs
	Duplicate bool `json:"duplicate,omitempty"`

	// Indicates whether or not a note is duplicated across Kinds
	DuplicateKind bool `json:"duplicate_kind,omitempty"`

	// ActionRequired indicates whether or not the release-note-action-required
	// label was set on the PR
	ActionRequired bool `json:"action_required,omitempty"`

	// DoNotPublish by default represents release-note-none label on GitHub
	DoNotPublish bool `json:"do_not_publish,omitempty"`

	// DataFields a key indexed map of data fields
	DataFields map[string]ReleaseNotesDataField `json:"-"`
}

type Documentation struct {
	// A description about the documentation
	Description string `json:"description,omitempty"`

	// The url to be linked
	URL string `json:"url"`

	// Classifies the link as something special, like a KEP
	Type DocType `json:"type"`
}

type DocType string

const (
	DocTypeExternal DocType = "external"
	DocTypeKEP      DocType = "KEP"
	DocTypeOfficial DocType = "official"
)

// ReleaseNotes is the main struct for collecting release notes
type ReleaseNotes struct {
	byPR    ReleaseNotesByPR
	history ReleaseNotesHistory
}

// NewReleaseNotes can be used to create a new empty ReleaseNotes struct
func NewReleaseNotes() *ReleaseNotes {
	return &ReleaseNotes{
		byPR: make(ReleaseNotesByPR),
	}
}

// ReleaseNotes is a map of PR numbers referencing notes.
// To avoid needless loops, we need to be able to reference things by PR
// When we have to merge old and new entries, we want to be able to override
// the old entries with the new ones efficiently.
type ReleaseNotesByPR map[int]*ReleaseNote

// ReleaseNotesHistory is the sorted list of PRs in the commit history
type ReleaseNotesHistory []int

// History returns the ReleaseNotesHistory for the ReleaseNotes
func (r *ReleaseNotes) History() ReleaseNotesHistory {
	return r.history
}

// ByPR returns the ReleaseNotesByPR for the ReleaseNotes
func (r *ReleaseNotes) ByPR() ReleaseNotesByPR {
	return r.byPR
}

// Get returns the ReleaseNote for the provided prNumber
func (r *ReleaseNotes) Get(prNumber int) *ReleaseNote {
	return r.byPR[prNumber]
}

// Set can be used to set a release note for the provided prNumber
func (r *ReleaseNotes) Set(prNumber int, note *ReleaseNote) {
	r.byPR[prNumber] = note
	r.history = append(r.history, prNumber)
}

type Result struct {
	commit      *gogithub.RepositoryCommit
	pullRequest *gogithub.PullRequest
}

type Gatherer struct {
	client       github.Client
	context      context.Context
	options      *options.Options
	MapProviders []*MapProvider
}

// NewGatherer creates a new notes gatherer
func NewGatherer(ctx context.Context, opts *options.Options) (*Gatherer, error) {
	client, err := opts.Client()
	if err != nil {
		return nil, errors.Wrap(err, "unable to create notes client")
	}
	return &Gatherer{
		client:  client,
		context: ctx,
		options: opts,
	}, nil
}

// NewGathererWithClient creates a new notes gatherer with a specific client
func NewGathererWithClient(ctx context.Context, c github.Client) *Gatherer {
	return &Gatherer{
		client:  c,
		context: ctx,
		options: options.New(),
	}
}

// GatherReleaseNotes creates a new gatherer and collects the release notes
// afterwards
func GatherReleaseNotes(opts *options.Options) (*ReleaseNotes, error) {
	logrus.Info("Gathering release notes")
	gatherer, err := NewGatherer(context.Background(), opts)
	if err != nil {
		return nil, errors.Wrapf(err, "retrieving notes gatherer")
	}

	var releaseNotes *ReleaseNotes
	startTime := time.Now()
	if gatherer.options.ListReleaseNotesV2 {
		logrus.Warn("EXPERIMENTAL IMPLEMENTATION ListReleaseNotesV2 ENABLED")
		releaseNotes, err = gatherer.ListReleaseNotesV2()
	} else {
		releaseNotes, err = gatherer.ListReleaseNotes()
	}
	if err != nil {
		return nil, errors.Wrapf(err, "listing release notes")
	}
	logrus.Infof("finished gathering release notes in %v", time.Since(startTime))

	return releaseNotes, nil
}

// ListReleaseNotes produces a list of fully contextualized release notes
// starting from a given commit SHA and ending at starting a given commit SHA.
func (g *Gatherer) ListReleaseNotes() (*ReleaseNotes, error) {
	// Load map providers
	mapProviders := []MapProvider{}
	for _, initString := range g.options.MapProviderStrings {
		provider, err := NewProviderFromInitString(initString)
		if err != nil {
			return nil, errors.Wrap(err, "while getting release notes map providers")
		}
		mapProviders = append(mapProviders, provider)
	}

	commits, err := g.listCommits(g.options.Branch, g.options.StartSHA, g.options.EndSHA)
	if err != nil {
		return nil, errors.Wrap(err, "listing commits")
	}

	// Get the PRs into a temporary results set
	resultsTemp, err := g.gatherNotes(commits)
	if err != nil {
		return nil, errors.Wrap(err, "gathering notes")
	}

	// Cycle the results and add the complete notes, as well as those that
	// have a map associated with it
	results := []*Result{}
	logrus.Info("Checking PRs for mapped data")
	for _, res := range resultsTemp {
		// If the PR has no release note, check if we have to add it
		if MatchesExcludeFilter(*res.pullRequest.Body) {
			for _, provider := range mapProviders {
				noteMaps, err := provider.GetMapsForPR(res.pullRequest.GetNumber())
				if err != nil {
					return nil, errors.Wrapf(
						err, "checking if a map exists for PR %d", res.pullRequest.GetNumber(),
					)
				}
				if len(noteMaps) != 0 {
					logrus.Infof(
						"Artificially adding pr #%d because a map for it was found",
						res.pullRequest.GetNumber(),
					)
					results = append(results, res)
				} else {
					logrus.Debugf(
						"Skipping PR #%d because it contains no release note",
						res.pullRequest.GetNumber(),
					)
				}
			}
		} else {
			// Append the note as it is
			results = append(results, res)
		}
	}

	dedupeCache := map[string]struct{}{}
	notes := NewReleaseNotes()
	for _, result := range results {
		if g.options.RequiredAuthor != "" {
			if result.commit.GetAuthor().GetLogin() != g.options.RequiredAuthor {
				logrus.Infof(
					"Skipping release note for PR #%d because required author %q does not match with %q",
					result.pullRequest.GetNumber(), g.options.RequiredAuthor, result.commit.GetAuthor().GetLogin(),
				)
				continue
			}
		}

		note, err := g.ReleaseNoteFromCommit(result)
		if err != nil {
			logrus.Errorf(
				"Getting the release note from commit %s (PR #%d): %v",
				result.commit.GetSHA(),
				result.pullRequest.GetNumber(),
				err)
			continue
		}

		// Query our map providers for additional data for the release note
		for _, provider := range mapProviders {
			noteMaps, err := provider.GetMapsForPR(result.pullRequest.GetNumber())
			if err != nil {
				return nil, errors.Wrap(err, "Error while looking up note map")
			}

			for _, noteMap := range noteMaps {
				if err := note.ApplyMap(noteMap, g.options.AddMarkdownLinks); err != nil {
					return nil, errors.Wrapf(err, "applying notemap for PR #%d", result.pullRequest.GetNumber())
				}
			}
		}
		if _, ok := dedupeCache[note.Text]; !ok {
			notes.Set(note.PrNumber, note)
			dedupeCache[note.Text] = struct{}{}
		}
	}
	return notes, nil
}

// noteTextFromString returns the text of the release note given a string which
// may contain the commit message, the PR description, etc.
// This is generally the content inside the ```release-note ``` stanza.
func noteTextFromString(s string) (string, error) {
	exps := []*regexp.Regexp{
		// (?s) is needed for '.' to be matching on newlines, by default that's disabled
		// we need to match ungreedy 'U', because after the notes a `docs` block can occur
		regexp.MustCompile("(?sU)```release-note[s]?\\r\\n(?P<note>.+)\\r\\n```"),
		regexp.MustCompile("(?sU)```dev-release-note[s]?\\r\\n(?P<note>.+)"),
		regexp.MustCompile("(?sU)```\\r\\n(?P<note>.+)\\r\\n```"),
		regexp.MustCompile("(?sU)```release-note[s]?\n(?P<note>.+)\n```"),
	}

	for _, exp := range exps {
		match := exp.FindStringSubmatch(s)
		if len(match) == 0 {
			continue
		}
		result := map[string]string{}
		for i, name := range exp.SubexpNames() {
			if i != 0 && name != "" {
				result[name] = match[i]
			}
		}

		note := strings.ReplaceAll(result["note"], "\r", "")
		note = stripActionRequired(note)
		note = dashify(note)
		note = unlist(note)
		note = strings.TrimSpace(note)
		return note, nil
	}

	return "", errors.New("no matches found when parsing note text from commit string")
}

func DocumentationFromString(s string) []*Documentation {
	regex := regexp.MustCompile("(?s)```docs[\\r]?\\n(?P<text>.+)[\\r]?\\n```")
	match := regex.FindStringSubmatch(s)

	if len(match) < 1 {
		// Nothing found, but we don't require it
		return nil
	}

	result := []*Documentation{}
	text := match[1]
	text = stripStar(text)
	text = stripDash(text)

	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		const httpPrefix = "http"
		s := strings.SplitN(scanner.Text(), httpPrefix, 2)
		if len(s) != 2 {
			continue
		}
		description := strings.TrimRight(strings.TrimSpace(s[0]), " :-")
		urlString := httpPrefix + strings.TrimSpace(s[1])

		// Validate the URL
		parsedURL, err := url.Parse(urlString)
		if err != nil {
			continue
		}

		result = append(result, &Documentation{
			Description: description,
			URL:         urlString,
			Type:        classifyURL(parsedURL),
		})
	}

	return result
}

// classifyURL returns the correct DocType for the given url
func classifyURL(u *url.URL) DocType {
	// Kubernetes Enhancement Proposals (KEPs)
	if strings.Contains(u.Host, "github.com") &&
		strings.Contains(u.Path, "/kubernetes/enhancements/") {
		return DocTypeKEP
	}

	// Official documentation
	if strings.Contains(u.Host, "kubernetes.io") &&
		strings.Contains(u.Path, "/docs/") {
		return DocTypeOfficial
	}

	return DocTypeExternal
}

// ReleaseNoteFromCommit produces a full contextualized release note given a
// GitHub commit API resource.
func (g *Gatherer) ReleaseNoteFromCommit(result *Result) (*ReleaseNote, error) {
	pr := result.pullRequest

	prBody := pr.GetBody()
	text, err := noteTextFromString(prBody)
	if err != nil {
		return nil, err
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

	// TODO: Spin this to sep function
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
		Commit:         result.commit.GetSHA(),
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

// listCommits lists all commits starting from a given commit SHA and ending at
// a given commit SHA.
func (g *Gatherer) listCommits(branch, start, end string) ([]*gogithub.RepositoryCommit, error) {
	startCommit, _, err := g.client.GetCommit(g.context, g.options.GithubOrg, g.options.GithubRepo, start)
	if err != nil {
		return nil, errors.Wrap(err, "retrieve start commit")
	}

	endCommit, _, err := g.client.GetCommit(g.context, g.options.GithubOrg, g.options.GithubRepo, end)
	if err != nil {
		return nil, errors.Wrap(err, "retrieve end commit")
	}

	allCommits := &commitList{}

	worker := func(clo *gogithub.CommitsListOptions) (
		commits []*gogithub.RepositoryCommit, resp *gogithub.Response, err error,
	) {
		for {
			commits, resp, err = g.client.ListCommits(g.context, g.options.GithubOrg, g.options.GithubRepo, clo)
			if err != nil {
				if !canWaitAndRetry(resp, err) {
					return nil, nil, err
				}
			} else {
				break
			}
		}
		return commits, resp, err
	}

	clo := gogithub.CommitsListOptions{
		SHA:   branch,
		Since: startCommit.GetCommitter().GetDate(),
		Until: endCommit.GetCommitter().GetDate(),
		ListOptions: gogithub.ListOptions{
			Page:    1,
			PerPage: 100,
		},
	}

	commits, resp, err := worker(&clo)
	if err != nil {
		return nil, err
	}
	allCommits.Add(commits)

	remainingPages := resp.LastPage - 1
	if remainingPages < 1 {
		return allCommits.List(), nil
	}

	t := throttler.New(maxParallelRequests, remainingPages)
	for page := 2; page <= resp.LastPage; page++ {
		clo := clo
		clo.ListOptions.Page = page

		go func() {
			commits, _, err := worker(&clo)
			if err == nil {
				allCommits.Add(commits)
			}
			t.Done(err)
		}()

		// abort all, if we got one error
		if t.Throttle() > 0 {
			break
		}
	}

	if err := t.Err(); err != nil {
		return nil, err
	}

	return allCommits.List(), nil
}

type commitList struct {
	sync.RWMutex
	list []*gogithub.RepositoryCommit
}

func (l *commitList) Add(c []*gogithub.RepositoryCommit) {
	l.Lock()
	defer l.Unlock()
	l.list = append(l.list, c...)
}

func (l *commitList) List() []*gogithub.RepositoryCommit {
	l.RLock()
	defer l.RUnlock()
	return l.list
}

// noteExclusionFilters is a list of regular expressions that match commits
// that do NOT contain release notes. Notably, this is all of the variations of
// "release note none" that appear in the commit log.
var noteExclusionFilters = []*regexp.Regexp{
	// 'none','n/a','na' case insensitive with optional trailing
	// whitespace, wrapped in ``` with/without release-note identifier
	// the 'none','n/a','na' can also optionally be wrapped in quotes ' or "
	regexp.MustCompile("(?i)```release-note[s]?\\s*('|\")?(none|n/a|na)?('|\")?\\s*```"),

	// simple '/release-note-none' tag
	regexp.MustCompile("/release-note-none"),
}

// Similarly, now that the known not-release-notes are filtered out, we can
// use some patterns to find actual release notes.
var noteInclusionFilters = []*regexp.Regexp{
	regexp.MustCompile("release-note"),
	regexp.MustCompile("Does this PR introduce a user-facing change?"),
}

// MatchesExcludeFilter returns true if the string matches an excluded release note
func MatchesExcludeFilter(msg string) bool {
	return matchesFilter(msg, noteExclusionFilters)
}

// MatchesIncludeFilter returns true if the string matches an included release note
func MatchesIncludeFilter(msg string) bool {
	return matchesFilter(msg, noteInclusionFilters)
}

func matchesFilter(msg string, filters []*regexp.Regexp) bool {
	for _, filter := range filters {
		if filter.MatchString(msg) {
			return true
		}
	}
	return false
}

// gatherNotes list commits that have release notes starting from a given
// commit SHA and ending at a given commit SHA. This function is similar to
// listCommits except that only commits with tagged release notes are returned.
func (g *Gatherer) gatherNotes(commits []*gogithub.RepositoryCommit) (filtered []*Result, err error) {
	allResults := &resultList{}

	nrOfCommits := len(commits)

	// A note about prallelism:
	//
	// We make 2 different requests to GitHub further down the stack:
	// - If we find PR numbers in a commit message, we do one API call per found
	//   number. The assumption is, that this is mostly just one (or just a couple
	//   of) PRs. The calls to the API do run in serial right now.
	// - If we don't find a PR number in the commit message, we ask the API if
	//   GitHub knows about PRs that are connected to that specific commit. The
	//   assumption again is that this is either one or just a couple of PRs. The
	//   results probably fit into one GitHub result page. If not, and we need to
	//   query multiple times (paging), we currently also do that in serial.
	//
	// In case we parallelize the above mentioned API calls and the volume of
	// them is bigger than expected, we might go well above the
	// `maxParallelRequests` of parallel requests. In that case we probably
	// should introduce the throttler as a global concept (on the Gatherer or so)
	// and use that throttler for all API calls.
	t := throttler.New(maxParallelRequests, nrOfCommits)

	notesForCommit := func(commit *gogithub.RepositoryCommit) {
		res, err := g.notesForCommit(commit)
		if err == nil && res != nil {
			allResults.Add(res)
		}
		t.Done(err)
	}

	for i, commit := range commits {
		logrus.Infof(
			"Starting to process commit %d of %d (%0.2f%%): %s",
			i+1, nrOfCommits, (float64(i+1)/float64(nrOfCommits))*100.0,
			commit.GetSHA(),
		)

		if g.options.ReplayDir == "" {
			go notesForCommit(commit)
		} else {
			// Ensure the same order like recorded
			notesForCommit(commit)
		}

		if t.Throttle() > 0 {
			break
		}
	} // for range commits

	if err := t.Err(); err != nil {
		return nil, err
	}

	return allResults.List(), nil
}

func (g *Gatherer) notesForCommit(commit *gogithub.RepositoryCommit) (*Result, error) {
	prs, err := g.prsFromCommit(commit)
	if err != nil {
		if err == errNoPRIDFoundInCommitMessage || err == errNoPRFoundForCommitSHA {
			logrus.Debugf(
				"No matches found when parsing PR from commit SHA %s",
				commit.GetSHA(),
			)
			return nil, nil
		}
		return nil, err
	}

	for _, pr := range prs {
		prBody := pr.GetBody()

		logrus.Debugf(
			"Got PR #%d for commit: %s", pr.GetNumber(), commit.GetSHA(),
		)

		if MatchesIncludeFilter(prBody) {
			res := &Result{commit: commit, pullRequest: pr}
			logrus.Infof("PR #%d seems to contain a release note", pr.GetNumber())
			// Do not test further PRs for this commit as soon as one PR matched
			return res, nil
		}
	}

	return nil, nil
}

type resultList struct {
	sync.RWMutex
	list []*Result
}

func (l *resultList) Add(r *Result) {
	l.Lock()
	defer l.Unlock()
	l.list = append(l.list, r)
}

func (l *resultList) List() []*Result {
	l.RLock()
	defer l.RUnlock()
	return l.list
}

// prsFromCommit return an API Pull Request struct given a commit struct. This is
// useful for going from a commit log to the PR (which contains useful info such
// as labels).
func (g *Gatherer) prsFromCommit(commit *gogithub.RepositoryCommit) (
	[]*gogithub.PullRequest, error,
) {
	githubPRs, err := g.prsForCommitFromMessage(*commit.Commit.Message)
	if err != nil {
		logrus.Debugf("No PR found for commit %s: %v", commit.GetSHA(), err)
		return g.prsForCommitFromSHA(*commit.SHA)
	}
	return githubPRs, err
}

// labelsWithPrefix is a helper for fetching all labels on a PR that start with
// a given string. This pattern is used often in the k/k repo and we can take
// advantage of this to contextualize release note generation with the kind, sig,
// area, etc labels.
func labelsWithPrefix(pr *gogithub.PullRequest, prefix string) []string {
	labels := []string{}
	for _, label := range pr.Labels {
		if strings.HasPrefix(*label.Name, prefix) {
			labels = append(labels, strings.TrimPrefix(*label.Name, prefix+"/"))
		}
	}
	return labels
}

// labelExactMatch indicates whether or not a matching label was found on PR
func labelExactMatch(pr *gogithub.PullRequest, labelToFind string) bool {
	for _, label := range pr.Labels {
		if *label.Name == labelToFind {
			return true
		}
	}
	return false
}

func stripActionRequired(note string) string {
	expressions := []string{
		`(?i)\[action required\]\s`,
		`(?i)action required:\s`,
	}

	for _, exp := range expressions {
		re := regexp.MustCompile(exp)
		note = re.ReplaceAllString(note, "")
	}

	return note
}

func stripStar(note string) string {
	re := regexp.MustCompile(`(?i)\*\s`)
	return re.ReplaceAllString(note, "")
}

func stripDash(note string) string {
	re := regexp.MustCompile(`(?i)\-\s`)
	return re.ReplaceAllString(note, "")
}

const listPrefix = "- "

func dashify(note string) string {
	re := regexp.MustCompile(`(?m)(^\s*)\*\s`)
	return re.ReplaceAllString(note, "$1- ")
}

// unlist transforms a single markdown list entry to a flat note entry
func unlist(note string) string {
	if !strings.HasPrefix(note, listPrefix) {
		return note
	}

	res := strings.Builder{}
	scanner := bufio.NewScanner(strings.NewReader(note))
	firstLine := true
	trim := true
	for scanner.Scan() {
		line := scanner.Text()

		// Per default strip the two dashes from the list
		prefix := "  "

		if strings.HasPrefix(line, listPrefix) {
			if firstLine {
				// First list item, strip the prefix
				prefix = listPrefix
				firstLine = false
			} else {
				// Another list item? Treat it as sublist and do not trim any
				// more.
				trim = false
			}
		}

		if trim {
			line = strings.TrimPrefix(line, prefix)
		}

		res.WriteString(line + "\n")
	}
	return res.String()
}

func hasString(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}

// canWaitAndRetry retruen true if the gatherer hit the GitHub API secondary rate limit
func canWaitAndRetry(r *gogithub.Response, err error) bool {
	// If we hit the secondary rate limit...
	if r == nil {
		return false
	}
	if r.StatusCode == http.StatusForbidden &&
		strings.Contains(err.Error(), "secondary rate limit. Please wait") {
		// ... sleep for a minute plus a random bit so that workers don't
		// respawn all at the same time
		rtime, err := rand.Int(rand.Reader, big.NewInt(30))
		if err != nil {
			logrus.Error(err)
			return false
		}
		waitTime := rtime.Int64() + apiSleepTime
		logrus.Warnf("Hit the GitHub secondary rate limit, sleeping for %d secs.", waitTime)
		time.Sleep(time.Duration(waitTime) * time.Second)
		return true
	}
	return false
}

// prsForCommitFromSHA retrieves the PR numbers for a commit given its sha
func (g *Gatherer) prsForCommitFromSHA(sha string) (prs []*gogithub.PullRequest, err error) {
	plo := &gogithub.PullRequestListOptions{
		State: "closed",
		ListOptions: gogithub.ListOptions{
			Page:    1,
			PerPage: 100,
		},
	}

	prs = []*gogithub.PullRequest{}
	var pResult []*gogithub.PullRequest
	var resp *gogithub.Response

	for {
		for {
			pResult, resp, err = g.client.ListPullRequestsWithCommit(
				g.context, g.options.GithubOrg, g.options.GithubRepo, sha, plo,
			)
			if err != nil {
				if !canWaitAndRetry(resp, err) {
					return nil, err
				}
			} else {
				break
			}
		}
		prs = append(prs, pResult...)
		if resp.NextPage == 0 {
			break
		}
		plo.ListOptions.Page++
		if plo.ListOptions.Page > resp.LastPage {
			break
		}
	}

	if len(prs) == 0 {
		return nil, errNoPRFoundForCommitSHA
	}

	return prs, nil
}

func (g *Gatherer) prsForCommitFromMessage(commitMessage string) (prs []*gogithub.PullRequest, err error) {
	prsNum, err := prsNumForCommitFromMessage(commitMessage)
	if err != nil {
		return nil, err
	}
	var res *gogithub.PullRequest
	var resp *gogithub.Response

	for _, pr := range prsNum {
		// Given the PR number that we've now converted to an integer, get the PR from
		// the API
		for {
			res, resp, err = g.client.GetPullRequest(g.context, g.options.GithubOrg, g.options.GithubRepo, pr)
			if err != nil {
				if !canWaitAndRetry(resp, err) {
					return nil, err
				}
			} else {
				break
			}
		}
		prs = append(prs, res)
	}

	return prs, err
}

func prsNumForCommitFromMessage(commitMessage string) (prs []int, err error) {
	// Thankfully k8s-merge-robot commits the PR number consistently. If this ever
	// stops being true, this definitely won't work anymore.
	regex := regexp.MustCompile(`Merge pull request #(?P<number>\d+)`)
	pr := prForRegex(regex, commitMessage)
	if pr != 0 {
		prs = append(prs, pr)
	}

	regex = regexp.MustCompile(`automated-cherry-pick-of-#(?P<number>\d+)`)
	pr = prForRegex(regex, commitMessage)
	if pr != 0 {
		prs = append(prs, pr)
	}

	// If the PR was squash merged, the regexp is different
	regex = regexp.MustCompile(`\(#(?P<number>\d+)\)`)
	pr = prForRegex(regex, commitMessage)
	if pr != 0 {
		prs = append(prs, pr)
	}

	if prs == nil {
		return nil, errNoPRIDFoundInCommitMessage
	}

	return prs, nil
}

func prForRegex(regex *regexp.Regexp, commitMessage string) int {
	result := map[string]string{}
	match := regex.FindStringSubmatch(commitMessage)

	if match == nil {
		return 0
	}

	for i, name := range regex.SubexpNames() {
		if i != 0 && name != "" {
			result[name] = match[i]
		}
	}

	pr, err := strconv.Atoi(result["number"])
	if err != nil {
		return 0
	}
	return pr
}

// prettySIG takes a sig name as parsed by the `sig-foo` label and returns a
// "pretty" version of it that can be printed in documents
func prettySIG(sig string) string {
	parts := strings.Split(sig, "-")
	for i, part := range parts {
		switch part {
		case "vsphere":
			parts[i] = "vSphere"
		case "vmware":
			parts[i] = "VMWare"
		case "openstack":
			parts[i] = "OpenStack"
		case "api", "aws", "cli", "gcp":
			parts[i] = strings.ToUpper(part)
		default:
			parts[i] = strings.Title(part)
		}
	}
	return strings.Join(parts, " ")
}

func prettifySIGList(sigs []string) string {
	sigList := ""

	// sort the list so that any group of SIGs with the same content gives us the
	// same result
	sort.Strings(sigs)

	for i, sig := range sigs {
		if i == 0 {
			sigList = fmt.Sprintf("SIG %s", prettySIG(sig))
		} else if i == len(sigs)-1 {
			sigList = fmt.Sprintf("%s and %s", sigList, prettySIG(sig))
		} else {
			sigList = fmt.Sprintf("%s, %s", sigList, prettySIG(sig))
		}
	}

	return sigList
}

// ApplyMap Modifies the content of the release using information from
//  a ReleaseNotesMap
func (rn *ReleaseNote) ApplyMap(noteMap *ReleaseNotesMap, markdownLinks bool) error {
	logrus.WithFields(logrus.Fields{
		"pr": rn.PrNumber,
	}).Debugf("Applying map to note")
	reRenderMarkdown := false
	if noteMap.ReleaseNote.Author != nil {
		rn.Author = *noteMap.ReleaseNote.Author
		rn.AuthorURL = "https://github.com/" + *noteMap.ReleaseNote.Author
		reRenderMarkdown = true
	}

	if noteMap.ReleaseNote.Text != nil {
		rn.Text = *noteMap.ReleaseNote.Text
		reRenderMarkdown = true
	}

	if noteMap.ReleaseNote.Documentation != nil {
		rn.Documentation = *noteMap.ReleaseNote.Documentation
	}

	if noteMap.ReleaseNote.Areas != nil {
		rn.Areas = *noteMap.ReleaseNote.Areas
	}

	if noteMap.ReleaseNote.Kinds != nil {
		rn.Kinds = *noteMap.ReleaseNote.Kinds
	}

	if noteMap.ReleaseNote.SIGs != nil {
		rn.SIGs = *noteMap.ReleaseNote.SIGs
	}

	if noteMap.ReleaseNote.Feature != nil {
		rn.Feature = *noteMap.ReleaseNote.Feature
	}

	if noteMap.ReleaseNote.ActionRequired != nil {
		rn.ActionRequired = *noteMap.ReleaseNote.ActionRequired
	}

	if noteMap.ReleaseNote.DoNotPublish != nil {
		rn.DoNotPublish = *noteMap.ReleaseNote.DoNotPublish
	}

	// If there are datafields, add them
	if len(noteMap.DataFields) > 0 {
		rn.DataFields = make(map[string]ReleaseNotesDataField)
	}
	for key, df := range noteMap.DataFields {
		rn.DataFields[key] = df
	}

	// If parts of the markup where modified, change them
	// TODO: Spin this to sep function
	if reRenderMarkdown {
		indented := strings.ReplaceAll(rn.Text, "\n", "\n  ")
		markdown := fmt.Sprintf("%s (#%d, @%s)",
			indented, rn.PrNumber, rn.Author)
		if markdownLinks {
			markdown = fmt.Sprintf("%s ([#%d](%s), [@%s](%s))",
				indented, rn.PrNumber, rn.PrURL, rn.Author, rn.AuthorURL)
		}
		// Uppercase the first character of the markdown to make it look uniform
		rn.Markdown = capitalizeString(markdown)
	}
	return nil
}

// ToNoteMap returns the note's content as YAML code for use in a notemap
func (rn *ReleaseNote) ToNoteMap() (string, error) {
	noteMap := &ReleaseNotesMap{
		PR:     rn.PrNumber,
		Commit: rn.Commit,
	}

	noteMap.ReleaseNote.Text = &rn.Text
	noteMap.ReleaseNote.Documentation = &rn.Documentation
	noteMap.ReleaseNote.Author = &rn.Author
	noteMap.ReleaseNote.Areas = &rn.Areas
	noteMap.ReleaseNote.Kinds = &rn.Kinds
	noteMap.ReleaseNote.SIGs = &rn.SIGs
	noteMap.ReleaseNote.Feature = &rn.Feature
	noteMap.ReleaseNote.ActionRequired = &rn.ActionRequired
	noteMap.ReleaseNote.DoNotPublish = &rn.DoNotPublish

	yamlCode, err := yaml.Marshal(&noteMap)
	if err != nil {
		return "", errors.Wrap(err, "marshalling release note to map")
	}

	return string(yamlCode), nil
}

// ContentHash returns a sha1 hash derived from the note's content
func (rn *ReleaseNote) ContentHash() (string, error) {
	// Convert the note to a map
	noteMap, err := rn.ToNoteMap()
	if err != nil {
		return "", errors.Wrap(err, "serializing note's content")
	}

	//nolint:gosec // used for file integrity checks, NOT security
	// TODO(relnotes): Could we use SHA256 here instead?
	h := sha1.New()
	_, err = h.Write([]byte(noteMap))
	if err != nil {
		return "", errors.Wrap(err, "calculating content hash from map")
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// capitalizeString returns a capitalized string of the input string
func capitalizeString(s string) string {
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])

	return string(r)
}
