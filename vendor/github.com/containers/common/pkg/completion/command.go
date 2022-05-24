package completion

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const (
	completionDescription = `Generate shell autocompletions.
Valid arguments are bash, zsh, fish and powershell.`

	bash       = "bash"
	zsh        = "zsh"
	fish       = "fish"
	powershell = "powershell"
)

var (
	file   string
	noDesc bool
	shells = []string{bash, zsh, fish, powershell}
)

// AddCompletionCommand adds the completion command to the given command which should be the root command.
// This command can be used the generate the cobra shell completion scripts for bash, zsh, fish and powershell.
func AddCompletionCommand(rootCmd *cobra.Command) {
	completionCmd := &cobra.Command{
		Use:       fmt.Sprintf("completion [options] {%s}", strings.Join(shells, "|")),
		Short:     "Generate shell autocompletions",
		Long:      completionDescription,
		ValidArgs: shells,
		Args:      cobra.ExactValidArgs(1),
		RunE:      completion,
		Example: fmt.Sprintf(`%[1]s completion bash
  %[1]s completion zsh -f _%[1]s
  %[1]s completion fish --no-desc`, rootCmd.Name()),
		// don't show this command to users
		Hidden: true,
	}

	flags := completionCmd.Flags()
	fileFlagName := "file"
	flags.StringVarP(&file, fileFlagName, "f", "", "Output the completion to file rather than stdout.")
	_ = completionCmd.RegisterFlagCompletionFunc(fileFlagName, AutocompleteDefault)

	flags.BoolVar(&noDesc, "no-desc", false, "Don't include descriptions in the completion output.")

	rootCmd.AddCommand(completionCmd)
}

func completion(cmd *cobra.Command, args []string) error {
	var w io.Writer

	if file != "" {
		file, err := os.Create(file)
		if err != nil {
			return err
		}
		defer file.Close()
		w = file
	} else {
		w = os.Stdout
	}

	var err error
	switch args[0] {
	case bash:
		err = cmd.Root().GenBashCompletionV2(w, !noDesc)
	case zsh:
		if noDesc {
			err = cmd.Root().GenZshCompletionNoDesc(w)
		} else {
			err = cmd.Root().GenZshCompletion(w)
		}
	case fish:
		err = cmd.Root().GenFishCompletion(w, !noDesc)
	case powershell:
		if noDesc {
			err = cmd.Root().GenPowerShellCompletion(w)
		} else {
			err = cmd.Root().GenPowerShellCompletionWithDesc(w)
		}
	}
	if err != nil {
		return err
	}

	_, err = io.WriteString(w, fmt.Sprintf(
		"# This file is generated with %q; DO NOT EDIT!\n", cmd.CommandPath(),
	))
	return err
}
