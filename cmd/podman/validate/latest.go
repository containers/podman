package validate

import (
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/spf13/cobra"
)

func AddLatestFlag(cmd *cobra.Command, b *bool) {
	// Initialization flag verification
	cmd.Flags().BoolVarP(b, "latest", "l", false,
		"Act on the latest container podman is aware of\nNot supported with the \"--remote\" flag")
	if registry.IsRemote() {
		_ = cmd.Flags().MarkHidden("latest")
	}
}
