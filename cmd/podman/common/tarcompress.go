package common

import (
	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/spf13/cobra"
)

func DefineTarCompressFlags(cmd *cobra.Command, tarCompFormat *string) {
	flags := cmd.Flags()

	tarCompFormatStr := "tar-compression-format"
	flags.StringVar(tarCompFormat, tarCompFormatStr, tarCompressionFormat(), "compression format to use for archives")
	_ = cmd.RegisterFlagCompletionFunc(tarCompFormatStr, AutocompleteTarCompressionFormat)

	tarCompLevel := "tar-compression-level"
	flags.Int(tarCompLevel, tarCompressionLevel(), "compression level to use for archives")
	_ = cmd.RegisterFlagCompletionFunc(tarCompLevel, completion.AutocompleteNone)

}

func tarCompressionFormat() string {
	if registry.IsRemote() {
		return ""
	}

	return podmanConfig.ContainersConfDefaultsRO.Engine.TarCompressionFormat
}

func tarCompressionLevel() int {
	if registry.IsRemote() || podmanConfig.ContainersConfDefaultsRO.Engine.TarCompressionLevel == nil {
		return 0
	}

	return *podmanConfig.ContainersConfDefaultsRO.Engine.TarCompressionLevel
}
