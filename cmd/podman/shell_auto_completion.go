package main

import (
	"os"

	"github.com/spf13/cobra"
)

const (
	bashAutoCompletionDescription = `To load completion run

podman completion bash >>~/.bashrc

Reload your shell.

To make it available in all your bash session, add this to your ~/.bashrc file:
echo 'source <(podman completion bash)' >>~/.bashrc

As an alternative, if 'bash-completion' is installed on your system, you can add it in:
/etc/bash_completion.d/{your_file_name}
`
)

var _bashCompletionCommand = &cobra.Command{
	Use:   "completion",
	Short: "Generates shell scripts for auto-completion",
	// We do not intend to show this command to the user so
	// we marked it as hidden. We should use them by using
	// the "make completion" target to update the shell scripts
	// enabling the auto-completion for podman.
	Hidden:  true,
	Example: `podman completion --help`,
}

func init() {
	_bashCompletionCommand.AddCommand(
		&cobra.Command{
			Use:   "bash",
			Short: "generate auto-completion for bash",
			Long:  bashAutoCompletionDescription,
			RunE: func(cmd *cobra.Command, args []string) error {
				// Writing the shell script to stdout allows the most flexible use
				// as the user can redirect the outputt where it needs it.
				return rootCmd.GenBashCompletion(os.Stdout)
			},
			Example: `podman completion bash >>~/.bashrc`,
		},
	)
}
