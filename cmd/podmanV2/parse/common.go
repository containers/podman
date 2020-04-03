package parse

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// CheckAllLatestAndCIDFile checks that --all and --latest are used correctly.
// If cidfile is set, also check for the --cidfile flag.
func CheckAllLatestAndCIDFile(c *cobra.Command, args []string, ignoreArgLen bool, cidfile bool) error {
	argLen := len(args)
	if c.Flags().Lookup("all") == nil || c.Flags().Lookup("latest") == nil {
		if !cidfile {
			return errors.New("unable to lookup values for 'latest' or 'all'")
		} else if c.Flags().Lookup("cidfile") == nil {
			return errors.New("unable to lookup values for 'latest', 'all' or 'cidfile'")
		}
	}

	specifiedAll, _ := c.Flags().GetBool("all")
	specifiedLatest, _ := c.Flags().GetBool("latest")
	specifiedCIDFile := false
	if cid, _ := c.Flags().GetStringArray("cidfile"); len(cid) > 0 {
		specifiedCIDFile = true
	}

	if specifiedCIDFile && (specifiedAll || specifiedLatest) {
		return errors.Errorf("--all, --latest and --cidfile cannot be used together")
	} else if specifiedAll && specifiedLatest {
		return errors.Errorf("--all and --latest cannot be used together")
	}

	if ignoreArgLen {
		return nil
	}
	if (argLen > 0) && (specifiedAll || specifiedLatest) {
		return errors.Errorf("no arguments are needed with --all or --latest")
	} else if cidfile && (argLen > 0) && (specifiedAll || specifiedLatest || specifiedCIDFile) {
		return errors.Errorf("no arguments are needed with --all, --latest or --cidfile")
	}

	if specifiedCIDFile {
		return nil
	}

	if argLen < 1 && !specifiedAll && !specifiedLatest && !specifiedCIDFile {
		return errors.Errorf("you must provide at least one name or id")
	}
	return nil
}
