package containers

import (
	"context"
	"os"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// runlabelOptionsWrapper allows for combining API-only with CLI-only options
// and to convert between them.
type runlabelOptionsWrapper struct {
	entities.ContainerRunlabelOptions
	TLSVerifyCLI bool
}

var (
	runlabelOptions     = runlabelOptionsWrapper{}
	runlabelDescription = "Executes a command as described by a container image label."
	runlabelCommand     = &cobra.Command{
		Use:   "runlabel [flags] LABEL IMAGE [ARG...]",
		Short: "Execute the command described by an image label",
		Long:  runlabelDescription,
		RunE:  runlabel,
		Args:  cobra.MinimumNArgs(2),
		Example: `podman container runlabel run imageID
  podman container runlabel --pull install imageID arg1 arg2
  podman container runlabel --display run myImage`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: runlabelCommand,
		Parent:  containerCmd,
	})

	flags := runlabelCommand.Flags()
	flags.StringVar(&runlabelOptions.Authfile, "authfile", auth.GetDefaultAuthFile(), "Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	flags.StringVar(&runlabelOptions.CertDir, "cert-dir", "", "`Pathname` of a directory containing TLS certificates and keys")
	flags.StringVar(&runlabelOptions.Credentials, "creds", "", "`Credentials` (USERNAME:PASSWORD) to use for authenticating to a registry")
	flags.BoolVar(&runlabelOptions.Display, "display", false, "Preview the command that the label would run")
	flags.StringVarP(&runlabelOptions.Name, "name", "n", "", "Assign a name to the container")
	flags.StringVar(&runlabelOptions.Optional1, "opt1", "", "Optional parameter to pass for install")
	flags.StringVar(&runlabelOptions.Optional2, "opt2", "", "Optional parameter to pass for install")
	flags.StringVar(&runlabelOptions.Optional3, "opt3", "", "Optional parameter to pass for install")
	flags.BoolP("pull", "p", false, "Pull the image if it does not exist locally prior to executing the label contents")
	flags.BoolVarP(&runlabelOptions.Quiet, "quiet", "q", false, "Suppress output information when installing images")
	flags.BoolVar(&runlabelOptions.Replace, "replace", false, "Replace existing container with a new one from the image")
	flags.StringVar(&runlabelOptions.SignaturePolicy, "signature-policy", "", "`Pathname` of signature policy file (not usually used)")
	flags.BoolVar(&runlabelOptions.TLSVerifyCLI, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")

	// Hide the optional flags.
	_ = flags.MarkHidden("opt1")
	_ = flags.MarkHidden("opt2")
	_ = flags.MarkHidden("opt3")
	_ = flags.MarkHidden("signature-policy")

	if err := flags.MarkDeprecated("pull", "podman will pull if not found in local storage"); err != nil {
		logrus.Error("unable to mark pull flag deprecated")
	}
}

func runlabel(cmd *cobra.Command, args []string) error {
	if cmd.Flags().Changed("tls-verify") {
		runlabelOptions.SkipTLSVerify = types.NewOptionalBool(!runlabelOptions.TLSVerifyCLI)
	}
	if runlabelOptions.Authfile != "" {
		if _, err := os.Stat(runlabelOptions.Authfile); err != nil {
			return errors.Wrapf(err, "error getting authfile %s", runlabelOptions.Authfile)
		}
	}
	return registry.ContainerEngine().ContainerRunlabel(context.Background(), args[0], args[1], args[2:], runlabelOptions.ContainerRunlabelOptions)
}
