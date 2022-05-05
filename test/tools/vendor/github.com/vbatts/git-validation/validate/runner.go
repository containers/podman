package validate

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/vbatts/git-validation/git"
)

// Runner is the for processing a set of rules against a range of commits
type Runner struct {
	Root        string
	Rules       []Rule
	Results     Results
	Verbose     bool
	CommitRange string // if this is empty, then it will default to FETCH_HEAD, then HEAD
}

// NewRunner returns an initiallized Runner.
func NewRunner(root string, rules []Rule, commitrange string, verbose bool) (*Runner, error) {
	newroot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path of %q: %s", root, err)
	}
	if commitrange == "" {
		var err error
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		defer os.Chdir(cwd)

		if err := os.Chdir(newroot); err != nil {
			return nil, err
		}
		commitrange, err = git.FetchHeadCommit()
		if err != nil {
			commitrange, err = git.HeadCommit()
			if err != nil {
				return nil, err
			}
		}
	}
	return &Runner{
		Root:        newroot,
		Rules:       rules,
		CommitRange: commitrange,
		Verbose:     verbose,
	}, nil
}

// Run processes the rules for each commit in the range provided
func (r *Runner) Run() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(cwd)

	if err := os.Chdir(r.Root); err != nil {
		return err
	}

	// collect the entries
	c, err := git.Commits(r.CommitRange)
	if err != nil {
		return err
	}

	// run them and show results
	for _, commit := range c {
		if os.Getenv("QUIET") == "" {
			fmt.Printf(" * %s %q ... ", commit["abbreviated_commit"], commit["subject"])
		}
		vr := Commit(commit, r.Rules)
		r.Results = append(r.Results, vr...)
		_, fail := vr.PassFail()
		if os.Getenv("QUIET") != "" {
			if fail != 0 {
				for _, res := range vr {
					if !res.Pass {
						fmt.Printf(" %s - FAIL - %s\n", commit["abbreviated_commit"], res.Msg)
					}
				}
			}
			// everything else in the loop is printing output.
			// If we're quiet, then just continue
			continue
		}
		if fail == 0 {
			fmt.Println("PASS")
		} else {
			fmt.Println("FAIL")
		}
		for _, res := range vr {
			if r.Verbose {
				if res.Pass {
					fmt.Printf("  - PASS - %s\n", res.Msg)
				} else {
					fmt.Printf("  - FAIL - %s\n", res.Msg)
				}
			} else if !res.Pass {
				fmt.Printf("  - FAIL - %s\n", res.Msg)
			}
		}
	}
	return nil
}
