package common

import (
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

// AddInspectFlagSet takes a command and adds the inspect flags and returns an InspectOptions object
// Since this cannot live in `package main` it lives here until a better home is found
func AddInspectFlagSet(cmd *cobra.Command) *entities.InspectOptions {
	opts := entities.InspectOptions{}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.Size, "size", "s", false, "Display total file size")
	flags.StringVarP(&opts.Format, "format", "f", "", "Change the output format to a Go template")

	return &opts
}
