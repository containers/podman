package validate

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// NoArgs returns an error if any args are included.
func NoArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("`%s` takes no arguments", cmd.CommandPath())
	}
	return nil
}

// SubCommandExists returns an error if no sub command is provided
func SubCommandExists(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		suggestions := cmd.SuggestionsFor(args[0])
		if len(suggestions) == 0 {
			return errors.Errorf("unrecognized command `%[1]s %[2]s`\nTry '%[1]s --help' for more information.", cmd.CommandPath(), args[0])
		}
		return errors.Errorf("unrecognized command `%[1]s %[2]s`\n\nDid you mean this?\n\t%[3]s\n\nTry '%[1]s --help' for more information.", cmd.CommandPath(), args[0], strings.Join(suggestions, "\n\t"))
	}
	cmd.Help()
	return errors.Errorf("missing command '%[1]s COMMAND'", cmd.CommandPath())
}

// IDOrLatestArgs used to validate a nameOrId was provided or the "--latest" flag
func IDOrLatestArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("`%s` accepts at most one argument", cmd.CommandPath())
	}

	latest := cmd.Flag("latest")
	if latest != nil {
		given, _ := strconv.ParseBool(cmd.Flag("latest").Value.String())
		if len(args) == 0 && !given {
			return fmt.Errorf("%q requires a name, id, or the \"--latest\" flag", cmd.CommandPath())
		}
		if len(args) > 0 && given {
			return fmt.Errorf("--latest and containers cannot be used together")
		}
	}
	return nil
}

// TODO: the two functions CheckAllLatestAndCIDFile and CheckAllLatestAndPodIDFile are almost identical.
//       It may be worth looking into generalizing the two a bit more and share code but time is scarce and
//       we only live once.

// CheckAllLatestAndCIDFile checks that --all and --latest are used correctly.
// If cidfile is set, also check for the --cidfile flag.
func CheckAllLatestAndCIDFile(c *cobra.Command, args []string, ignoreArgLen bool, cidfile bool) error {
	var specifiedLatest bool
	argLen := len(args)
	if !registry.IsRemote() {
		specifiedLatest, _ = c.Flags().GetBool("latest")
		if c.Flags().Lookup("all") == nil || c.Flags().Lookup("latest") == nil {
			if !cidfile {
				return errors.New("unable to lookup values for 'latest' or 'all'")
			} else if c.Flags().Lookup("cidfile") == nil {
				return errors.New("unable to lookup values for 'latest', 'all' or 'cidfile'")
			}
		}
	}

	specifiedAll, _ := c.Flags().GetBool("all")
	specifiedCIDFile := false
	if cid, _ := c.Flags().GetStringArray("cidfile"); len(cid) > 0 {
		specifiedCIDFile = true
	}

	if specifiedCIDFile && (specifiedAll || specifiedLatest) {
		return errors.Errorf("--all, --latest and --cidfile cannot be used together")
	} else if specifiedAll && specifiedLatest {
		return errors.Errorf("--all and --latest cannot be used together")
	}

	if (argLen > 0) && specifiedAll {
		return errors.Errorf("no arguments are needed with --all")
	}

	if ignoreArgLen {
		return nil
	}

	if argLen > 0 {
		if specifiedLatest {
			return errors.Errorf("--latest and containers cannot be used together")
		} else if cidfile && (specifiedLatest || specifiedCIDFile) {
			return errors.Errorf("no arguments are needed with --latest or --cidfile")
		}
	}

	if specifiedCIDFile {
		return nil
	}

	if argLen < 1 && !specifiedAll && !specifiedLatest && !specifiedCIDFile {
		return errors.Errorf("you must provide at least one name or id")
	}
	return nil
}

// CheckAllLatestAndPodIDFile checks that --all and --latest are used correctly.
// If withIDFile is set, also check for the --pod-id-file flag.
func CheckAllLatestAndPodIDFile(c *cobra.Command, args []string, ignoreArgLen bool, withIDFile bool) error {
	var specifiedLatest bool
	argLen := len(args)
	if !registry.IsRemote() {
		// remote clients have no latest flag
		specifiedLatest, _ = c.Flags().GetBool("latest")
		if c.Flags().Lookup("all") == nil || c.Flags().Lookup("latest") == nil {
			if !withIDFile {
				return errors.New("unable to lookup values for 'latest' or 'all'")
			} else if c.Flags().Lookup("pod-id-file") == nil {
				return errors.New("unable to lookup values for 'latest', 'all' or 'pod-id-file'")
			}
		}
	}

	specifiedAll, _ := c.Flags().GetBool("all")
	specifiedPodIDFile := false
	if pid, _ := c.Flags().GetStringArray("pod-id-file"); len(pid) > 0 {
		specifiedPodIDFile = true
	}

	if specifiedPodIDFile && (specifiedAll || specifiedLatest) {
		return errors.Errorf("--all, --latest and --pod-id-file cannot be used together")
	} else if specifiedAll && specifiedLatest {
		return errors.Errorf("--all and --latest cannot be used together")
	}

	if (argLen > 0) && specifiedAll {
		return errors.Errorf("no arguments are needed with --all")
	}

	if ignoreArgLen {
		return nil
	}

	if argLen > 0 {
		if specifiedLatest {
			return errors.Errorf("--latest and pods cannot be used together")
		} else if withIDFile && (specifiedLatest || specifiedPodIDFile) {
			return errors.Errorf("no arguments are needed with --latest or --pod-id-file")
		}
	}

	if specifiedPodIDFile {
		return nil
	}

	if argLen < 1 && !specifiedAll && !specifiedLatest && !specifiedPodIDFile {
		return errors.Errorf("you must provide at least one name or id")
	}
	return nil
}
