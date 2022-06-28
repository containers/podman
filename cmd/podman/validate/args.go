package validate

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/containers/podman/v4/cmd/podman/registry"
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
			return fmt.Errorf("unrecognized command `%[1]s %[2]s`\nTry '%[1]s --help' for more information", cmd.CommandPath(), args[0])
		}
		return fmt.Errorf("unrecognized command `%[1]s %[2]s`\n\nDid you mean this?\n\t%[3]s\n\nTry '%[1]s --help' for more information", cmd.CommandPath(), args[0], strings.Join(suggestions, "\n\t"))
	}
	cmd.Help() //nolint: errcheck
	return fmt.Errorf("missing command '%[1]s COMMAND'", cmd.CommandPath())
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
			return errors.New("--latest and containers cannot be used together")
		}
	}
	return nil
}

// CheckAllLatestAndCIDFile checks that --all and --latest are used correctly for containers and pods
// If idFileFlag is set is set, also checks for the --cidfile or --pod-id-file flag.
// Note: this has been deprecated, use CheckAllLatestAndIDFile instead
func CheckAllLatestAndCIDFile(c *cobra.Command, args []string, ignoreArgLen bool, cidfile bool) error {
	return CheckAllLatestAndIDFile(c, args, ignoreArgLen, "cidfile")
}

// CheckAllLatestAndPodIDFile checks that --all and --latest are used correctly.
// If withIDFile is set, also check for the --pod-id-file flag.
// Note: this has been deprecated, use CheckAllLatestAndIDFile instead
func CheckAllLatestAndPodIDFile(c *cobra.Command, args []string, ignoreArgLen bool, withIDFile bool) error {
	return CheckAllLatestAndIDFile(c, args, ignoreArgLen, "pod-id-file")
}

// CheckAllLatestAndIDFile checks that --all and --latest are used correctly for containers and pods
// If idFileFlag is set is set, also checks for the --cidfile or --pod-id-file flag.
func CheckAllLatestAndIDFile(c *cobra.Command, args []string, ignoreArgLen bool, idFileFlag string) error {
	var specifiedLatest bool
	argLen := len(args)
	if !registry.IsRemote() {
		specifiedLatest, _ = c.Flags().GetBool("latest")
		if c.Flags().Lookup("all") == nil || c.Flags().Lookup("latest") == nil {
			if idFileFlag == "" {
				return errors.New("unable to look up values for 'latest' or 'all'")
			} else if c.Flags().Lookup(idFileFlag) == nil {
				return fmt.Errorf("unable to look up values for 'latest', 'all', or '%s'", idFileFlag)
			}
		}
	}

	specifiedAll, _ := c.Flags().GetBool("all")
	specifiedIDFile := false
	if cid, _ := c.Flags().GetStringArray(idFileFlag); len(cid) > 0 {
		specifiedIDFile = true
	}

	if c.Flags().Changed("filter") {
		if argLen > 0 {
			return errors.New("--filter takes no arguments")
		}
		return nil
	}

	if specifiedIDFile && (specifiedAll || specifiedLatest) {
		return fmt.Errorf("--all, --latest, and --%s cannot be used together", idFileFlag)
	} else if specifiedAll && specifiedLatest {
		return errors.New("--all and --latest cannot be used together")
	}

	if (argLen > 0) && specifiedAll {
		return errors.New("no arguments are needed with --all")
	}

	if ignoreArgLen {
		return nil
	}

	if argLen > 0 {
		if specifiedLatest {
			return errors.New("--latest and containers cannot be used together")
		} else if idFileFlag != "" && (specifiedLatest || specifiedIDFile) {
			return fmt.Errorf("no arguments are needed with --latest or --%s", idFileFlag)
		}
	}

	if specifiedIDFile {
		return nil
	}

	if argLen < 1 && !specifiedAll && !specifiedLatest && !specifiedIDFile {
		return errors.New("you must provide at least one name or id")
	}
	return nil
}
