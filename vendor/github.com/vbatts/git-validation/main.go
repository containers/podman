package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/vbatts/git-validation/rules/danglingwhitespace"
	_ "github.com/vbatts/git-validation/rules/dco"
	_ "github.com/vbatts/git-validation/rules/messageregexp"
	_ "github.com/vbatts/git-validation/rules/shortsubject"
	"github.com/vbatts/git-validation/validate"
)

var (
	flCommitRange  = flag.String("range", "", "use this commit range instead (implies -no-travis)")
	flListRules    = flag.Bool("list-rules", false, "list the rules registered")
	flRun          = flag.String("run", "", "comma delimited list of rules to run. Defaults to all.")
	flVerbose      = flag.Bool("v", false, "verbose")
	flDebug        = flag.Bool("D", false, "debug output")
	flQuiet        = flag.Bool("q", false, "less output")
	flDir          = flag.String("d", ".", "git directory to validate from")
	flNoTravis     = flag.Bool("no-travis", false, "disables travis environment checks (when env TRAVIS=true is set)")
	flTravisPROnly = flag.Bool("travis-pr-only", true, "when on travis, only run validations if the CI-Build is checking pull-request build")
)

func main() {
	flag.Parse()

	if *flDebug {
		os.Setenv("DEBUG", "1")
	}
	if *flQuiet {
		os.Setenv("QUIET", "1")
	}

	if *flListRules {
		for _, r := range validate.RegisteredRules {
			fmt.Printf("%q -- %s\n", r.Name, r.Description)
		}
		return
	}

	if *flTravisPROnly && strings.ToLower(os.Getenv("TRAVIS_PULL_REQUEST")) == "false" {
		fmt.Printf("only to check travis PR builds and this not a PR build. yielding.\n")
		return
	}

	// rules to be used
	var rules []validate.Rule
	for _, r := range validate.RegisteredRules {
		// only those that are Default
		if r.Default {
			rules = append(rules, r)
		}
	}
	// or reduce the set being run to what the user provided
	if *flRun != "" {
		rules = validate.FilterRules(validate.RegisteredRules, validate.SanitizeFilters(*flRun))
	}
	if os.Getenv("DEBUG") != "" {
		log.Printf("%#v", rules) // XXX maybe reduce this list
	}

	var commitRange = *flCommitRange
	if commitRange == "" {
		if strings.ToLower(os.Getenv("TRAVIS")) == "true" && !*flNoTravis {
			if os.Getenv("TRAVIS_COMMIT_RANGE") != "" {
				commitRange = strings.Replace(os.Getenv("TRAVIS_COMMIT_RANGE"), "...", "..", 1)
			} else if os.Getenv("TRAVIS_COMMIT") != "" {
				commitRange = os.Getenv("TRAVIS_COMMIT")
			}
		}
	}

	runner, err := validate.NewRunner(*flDir, rules, commitRange, *flVerbose)
	if err != nil {
		log.Fatal(err)
	}

	if err := runner.Run(); err != nil {
		log.Fatal(err)
	}
	_, fail := runner.Results.PassFail()
	if fail > 0 {
		fmt.Printf("%d commits to fix\n", fail)
		os.Exit(1)
	}

}
