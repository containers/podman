package completion

import (
	"fmt"
	"io"
	"os"
	"strings"

	commonComp "github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/spf13/cobra"
)

const (
	completionDescription = `Generate shell autocompletions.
  Valid arguments are bash, zsh, and fish.
  Please refer to the man page to see how you can load these completions.`
)

var (
	file          string
	noDesc        bool
	shells        = []string{"bash", "zsh", "fish"}
	completionCmd = &cobra.Command{
		Use:       fmt.Sprintf("completion [options] {%s}", strings.Join(shells, "|")),
		Short:     "Generate shell autocompletions",
		Long:      completionDescription,
		ValidArgs: shells,
		Args:      cobra.ExactValidArgs(1),
		RunE:      completion,
		Example: `podman completion bash
  podman completion zsh -f _podman
  podman completion fish --no-desc`,
		//don't show this command to users
		Hidden: true,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: completionCmd,
	})
	flags := completionCmd.Flags()
	fileFlagName := "file"
	flags.StringVarP(&file, fileFlagName, "f", "", "Output the completion to file rather than stdout.")
	_ = completionCmd.RegisterFlagCompletionFunc(fileFlagName, commonComp.AutocompleteDefault)

	flags.BoolVar(&noDesc, "no-desc", false, "Don't include descriptions in the completion output.")
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
	case "bash":
		err = cmd.Root().GenBashCompletion(w)
	case "zsh":
		if noDesc {
			err = cmd.Root().GenZshCompletionNoDesc(w)
		} else {
			err = cmd.Root().GenZshCompletion(w)
		}
	case "fish":
		err = cmd.Root().GenFishCompletion(w, !noDesc)
	}

	if err != nil {
		return err
	}

	_, err = io.WriteString(w, fmt.Sprintf(
		"\n# This file is generated with %q; see: podman-completion(1)\n", cmd.CommandPath(),
	))
	return err
}
