package completion

import "github.com/spf13/cobra"

// FlagCompletions - hold flag completion functions to be applied later with CompleteCommandFlags()
type FlagCompletions map[string]func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

// CompleteCommandFlags - Add completion functions for each flagname in FlagCompletions.
func CompleteCommandFlags(cmd *cobra.Command, flags FlagCompletions) {
	for flagName, completionFunc := range flags {
		_ = cmd.RegisterFlagCompletionFunc(flagName, completionFunc)
	}
}

/* Autocomplete Functions for cobra ValidArgsFunction */

// AutocompleteNone - Block the default shell completion (no paths)
func AutocompleteNone(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteDefault - Use the default shell completion,
// allows path completion.
func AutocompleteDefault(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveDefault
}
