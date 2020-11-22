package validate

import (
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// NoArgs returns an error if any args are included.
func NoArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return errors.Errorf("`%s` takes no arguments", cmd.CommandPath())
	}
	return nil
}

// SubCommandExists returns an error if no sub command is provided
func SubCommandExists(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return errors.Errorf("unrecognized command `%[1]s %[2]s`\nTry '%[1]s --help' for more information.", cmd.CommandPath(), args[0])
	}
	return errors.Errorf("missing command '%[1]s COMMAND'\nTry '%[1]s --help' for more information.", cmd.CommandPath())
}

// OneContainerOrLatestArgs used to validate if one arg was provided or the "--latest" flag
func OneContainerOrLatestArgs(cmd *cobra.Command, args []string) error {
	return checkArgsOrLatest(cmd, args, true)
}

// ContainersOrLatestArgs used to validate if args are provided or the "--latest" flag
func ContainersOrLatestArgs(cmd *cobra.Command, args []string) error {
	return checkArgsOrLatest(cmd, args, false)
}

func checkArgsOrLatest(cmd *cobra.Command, args []string, singleArg bool) error {
	if singleArg && len(args) > 1 {
		return errors.Errorf("`%s` accepts at most one argument", cmd.CommandPath())
	}

	// This function is used by container and pod commands.
	// Lets make the error more descriptive by adding the correct noun to the error message.
	argType := "container"
	if cmd.Parent().Name() == "pod" {
		argType = "pod"
	}

	latest := cmd.Flag("latest")
	if latest != nil {
		given, _ := cmd.Flags().GetBool("latest")
		if len(args) == 0 && !given {
			return errors.Errorf("%q requires a %s name, id, or the \"--latest\" flag", cmd.CommandPath(), argType)
		}
		if len(args) > 0 && given {
			return errors.Errorf("--latest and %ss cannot be used together", argType)
		}
	}
	return nil
}

// ImagesOrAllArgs used to validate if args are provided or the "--all" flag
func ImagesOrAllArgs(cmd *cobra.Command, args []string) error {
	return checkAllArgs(cmd, args, "image")
}

// VolumesOrAllArgs used to validate if args are provided or the "--all" flag
func VolumesOrAllArgs(cmd *cobra.Command, args []string) error {
	return checkAllArgs(cmd, args, "volume")
}

// checkAllArgs used to validate if args are provided or the "--all" flag
func checkAllArgs(cmd *cobra.Command, args []string, name string) error {
	all := cmd.Flag("all")
	if all != nil {
		given, _ := cmd.Flags().GetBool("all")
		if len(args) == 0 && !given {
			return errors.Errorf("%q requires a %s name, id or the \"--all\" flag", cmd.CommandPath(), name)
		}
		if len(args) > 0 && given {
			return errors.Errorf("--all and %ss cannot be used together", name)
		}
	}
	return nil
}

// CheckStatOptions stats is different in that it will assume running
// containers if no input is given, so we need to validate differently
func CheckStatOptions(cmd *cobra.Command, args []string) error {
	opts := 0
	all, err := cmd.Flags().GetBool("all")
	if err != nil {
		return err
	}
	if all {
		opts++
	}
	if !registry.IsRemote() {
		latest, err := cmd.Flags().GetBool("latest")
		if err != nil {
			return err
		}
		if latest {
			opts++
		}
	}
	if len(args) > 0 {
		opts++
	}
	if opts > 1 {
		ctype := "container"
		if cmd.Parent().Name() == "pod" {
			ctype = "pod"
		}
		return errors.Errorf("--all, --latest and %ss cannot be used together", ctype)
	}
	return nil
}

// CheckAllLatestAndCIDFile checks that --all and --latest are used correctly.
// If cidfile is set, also check for the --cidfile flag.
func CheckAllLatestAndCIDFile(c *cobra.Command, args []string, ignoreArgLen bool, cidfile bool) error {
	return checkAllLatestAndIDFile(c, args, ignoreArgLen, cidfile, "cidfile")
}

// CheckAllLatestAndPodIDFile checks that --all and --latest are used correctly.
// If withIDFile is set, also check for the --pod-id-file flag.
func CheckAllLatestAndPodIDFile(c *cobra.Command, args []string, ignoreArgLen bool, withIDFile bool) error {
	return checkAllLatestAndIDFile(c, args, ignoreArgLen, withIDFile, "pod-id-file")
}

func checkAllLatestAndIDFile(c *cobra.Command, args []string, ignoreArgLen bool, withIDFile bool, idFlagName string) error {
	var specifiedLatest bool
	argLen := len(args)
	if !registry.IsRemote() {
		specifiedLatest, _ = c.Flags().GetBool("latest")
		if c.Flags().Lookup("all") == nil || c.Flags().Lookup("latest") == nil {
			if !withIDFile {
				return errors.New("unable to lookup values for 'latest' or 'all'")
			} else if c.Flags().Lookup(idFlagName) == nil {
				return errors.Errorf("unable to lookup values for 'latest', 'all' or '%s'", idFlagName)
			}
		}
	}

	specifiedAll, _ := c.Flags().GetBool("all")
	specifiedIDFile := false
	if cid, _ := c.Flags().GetStringArray(idFlagName); len(cid) > 0 {
		specifiedIDFile = true
	}

	if specifiedIDFile && (specifiedAll || specifiedLatest) {
		return errors.Errorf("--all, --latest and --%s cannot be used together", idFlagName)
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
		} else if withIDFile && (specifiedLatest || specifiedIDFile) {
			return errors.Errorf("no arguments are needed with --latest or --%s", idFlagName)
		}
	}

	if specifiedIDFile {
		return nil
	}

	if argLen < 1 && !specifiedAll && !specifiedLatest && !specifiedIDFile {
		return errors.Errorf("you must provide at least one name or id")
	}
	return nil
}
