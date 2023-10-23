package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	version "github.com/hashicorp/go-version"
	"github.com/sirupsen/logrus"
)

// Commits returns a set of commits.
// If commitrange is a git still range 12345...54321, then it will be isolated set of commits.
// If commitrange is a single commit, all ancestor commits up through the hash provided.
// If commitrange is an empty commit range, then nil is returned.
func Commits(commitrange string) ([]CommitEntry, error) {
	// TEST if it has _enough_ of a range from rev-list
	commitrange, err := checkRevList(commitrange)
	if err != nil {
		logrus.Errorf("failed to validate the git commit range: %s", err)
		return nil, err
	}
	cmdArgs := []string{"git", "rev-list", commitrange}
	if debug() {
		logrus.Infof("[git] cmd: %q", strings.Join(cmdArgs, " "))
	}
	output, err := exec.Command(cmdArgs[0], cmdArgs[1:]...).Output()
	if err != nil {
		logrus.Errorf("[git] cmd: %q", strings.Join(cmdArgs, " "))
		return nil, err
	}
	if len(output) == 0 {
		return nil, nil
	}
	commitHashes := strings.Split(strings.TrimSpace(string(output)), "\n")
	commits := make([]CommitEntry, len(commitHashes))
	for i, commitHash := range commitHashes {
		c, err := LogCommit(commitHash)
		if err != nil {
			return commits, err
		}
		commits[i] = *c
	}
	return commits, nil
}

// Since the commitrange requested may be longer than the depth being cloned in CI,
// check for an error, if so do a git log to get the oldest available commit for a reduced range.
func checkRevList(commitrange string) (string, error) {
	cmdArgs := []string{"git", "rev-list", commitrange}
	if debug() {
		logrus.Infof("[git] cmd: %q", strings.Join(cmdArgs, " "))
	}
	_, err := exec.Command(cmdArgs[0], cmdArgs[1:]...).Output()
	if err == nil {
		// no issues, return now
		return commitrange, nil
	}
	cmdArgs = []string{"git", "log", "--pretty=oneline"}
	if debug() {
		logrus.Infof("[git] cmd: %q", strings.Join(cmdArgs, " "))
	}
	output, err := exec.Command(cmdArgs[0], cmdArgs[1:]...).Output()
	if err != nil {
		logrus.Errorf("[git] cmd: %q", strings.Join(cmdArgs, " "))
		return "", err
	}
	// This "output" is now the list of available commits and short description.
	// We want the last commit hash only.. (i.e. `| tail -n1 | awk '{ print $1 }'`)
	chunks := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(chunks) == 1 {
		return strings.Split(chunks[0], " ")[0], nil
	}
	last := chunks[len(chunks)-1]
	lastCommit := strings.Split(last, " ")[0]

	return fmt.Sprintf("%s..HEAD", lastCommit), nil
}

// FieldNames are for the formating and rendering of the CommitEntry structs.
// Keys here are from git log pretty format "format:..."
var FieldNames = map[string]string{
	"%h":  "abbreviated_commit",
	"%p":  "abbreviated_parent",
	"%t":  "abbreviated_tree",
	"%aD": "author_date",
	"%aE": "author_email",
	"%aN": "author_name",
	"%b":  "body",
	"%H":  "commit",
	"%N":  "commit_notes",
	"%cD": "committer_date",
	"%cE": "committer_email",
	"%cN": "committer_name",
	"%e":  "encoding",
	"%P":  "parent",
	"%D":  "refs",
	"%f":  "sanitized_subject_line",
	"%GS": "signer",
	"%GK": "signer_key",
	"%s":  "subject",
	"%G?": "verification_flag",
}

func gitVersion() (string, error) {
	cmd := exec.Command("git", "version")
	cmd.Stderr = os.Stderr
	buf, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.Fields(string(buf))[2], nil
}

// https://github.com/vbatts/git-validation/issues/37
var versionWithExcludes = "1.9.5"

func gitVersionNewerThan(otherV string) (bool, error) {
	gv, err := gitVersion()
	if err != nil {
		return false, err
	}
	v1, err := version.NewVersion(gv)
	if err != nil {
		return false, err
	}
	v2, err := version.NewVersion(otherV)
	if err != nil {
		return false, err
	}
	return v2.Equal(v1) || v2.LessThan(v1), nil
}

// Check warns if changes introduce whitespace errors.
// Returns non-zero if any issues are found.
func Check(commit string) ([]byte, error) {
	args := []string{
		"--no-pager", "log", "--check",
		fmt.Sprintf("%s^..%s", commit, commit),
	}
	if excludeEnvList := os.Getenv("GIT_CHECK_EXCLUDE"); excludeEnvList != "" {
		gitNewEnough, err := gitVersionNewerThan(versionWithExcludes)
		if err != nil {
			return nil, err
		}
		if gitNewEnough {
			excludeList := strings.Split(excludeEnvList, ":")
			for _, exclude := range excludeList {
				if exclude == "" {
					continue
				}
				args = append(args, "--", ".", fmt.Sprintf(":(exclude)%s", exclude))
			}
		}
	}
	cmd := exec.Command("git", args...)
	if debug() {
		logrus.Infof("[git] cmd: %q", strings.Join(cmd.Args, " "))
	}
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

// Show returns the diff of a commit.
//
// NOTE: This could be expensive for very large commits.
func Show(commit string) ([]byte, error) {
	cmd := exec.Command("git", "--no-pager", "show", commit)
	if debug() {
		logrus.Infof("[git] cmd: %q", strings.Join(cmd.Args, " "))
	}
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

// CommitEntry represents a single commit's information from `git`.
// See also FieldNames
type CommitEntry map[string]string

// LogCommit assembles the full information on a commit from its commit hash
func LogCommit(commit string) (*CommitEntry, error) {
	c := CommitEntry{}
	for k, v := range FieldNames {
		cmd := exec.Command("git", "--no-pager", "log", "-1", `--pretty=format:`+k+``, commit)
		if debug() {
			logrus.Infof("[git] cmd: %q", strings.Join(cmd.Args, " "))
		}
		cmd.Stderr = os.Stderr
		out, err := cmd.Output()
		if err != nil {
			logrus.Errorf("[git] cmd: %q", strings.Join(cmd.Args, " "))
			return nil, err
		}
		commitMessage := strings.ReplaceAll(string(out), "\r\n", "\n")
		c[v] = strings.TrimSpace(commitMessage)
	}

	return &c, nil
}

func debug() bool {
	return len(os.Getenv("DEBUG")) > 0
}

// FetchHeadCommit returns the hash of FETCH_HEAD
func FetchHeadCommit() (string, error) {
	cmdArgs := []string{"git", "--no-pager", "rev-parse", "--verify", "FETCH_HEAD"}
	if debug() {
		logrus.Infof("[git] cmd: %q", strings.Join(cmdArgs, " "))
	}
	output, err := exec.Command(cmdArgs[0], cmdArgs[1:]...).Output()
	if err != nil {
		logrus.Errorf("[git] cmd: %q", strings.Join(cmdArgs, " "))
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// HeadCommit returns the hash of HEAD
func HeadCommit() (string, error) {
	cmdArgs := []string{"git", "--no-pager", "rev-parse", "--verify", "HEAD"}
	if debug() {
		logrus.Infof("[git] cmd: %q", strings.Join(cmdArgs, " "))
	}
	output, err := exec.Command(cmdArgs[0], cmdArgs[1:]...).Output()
	if err != nil {
		logrus.Errorf("[git] cmd: %q", strings.Join(cmdArgs, " "))
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
