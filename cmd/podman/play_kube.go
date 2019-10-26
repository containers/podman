package main

import (
	"fmt"
	"os"

	buildahcli "github.com/containers/buildah/pkg/cli"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	playKubeCommand     cliconfig.KubePlayValues
	playKubeDescription = `Command reads in a structured file of Kubernetes YAML.

  It creates the pod and containers described in the YAML.  The containers within the pod are then started and the ID of the new Pod is output.`
	_playKubeCommand = &cobra.Command{
		Use:   "kube [flags] KUBEFILE",
		Short: "Play a pod based on Kubernetes YAML",
		Long:  playKubeDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			playKubeCommand.InputArgs = args
			playKubeCommand.GlobalFlags = MainGlobalOpts
			playKubeCommand.Remote = remoteclient
			return playKubeCmd(&playKubeCommand)
		},
		Example: `podman play kube demo.yml`,
	}
)

func init() {
	if !remote {
		_playKubeCommand.Example = fmt.Sprintf("%s\n  podman play kube --cert-dir /mycertsdir --tls-verify=true --quiet myWebPod", _playKubeCommand.Example)
	}
	playKubeCommand.Command = _playKubeCommand
	playKubeCommand.SetHelpTemplate(HelpTemplate())
	playKubeCommand.SetUsageTemplate(UsageTemplate())
	flags := playKubeCommand.Flags()
	flags.StringVar(&playKubeCommand.Creds, "creds", "", "`Credentials` (USERNAME:PASSWORD) to use for authenticating to a registry")
	flags.BoolVarP(&playKubeCommand.Quiet, "quiet", "q", false, "Suppress output information when pulling images")
	// Disabled flags for the remote client
	if !remote {
		flags.StringVar(&playKubeCommand.Authfile, "authfile", buildahcli.GetDefaultAuthFile(), "Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
		flags.StringVar(&playKubeCommand.CertDir, "cert-dir", "", "`Pathname` of a directory containing TLS certificates and keys")
		flags.StringVar(&playKubeCommand.SignaturePolicy, "signature-policy", "", "`Pathname` of signature policy file (not usually used)")
		flags.BoolVar(&playKubeCommand.TlsVerify, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")
		markFlagHidden(flags, "signature-policy")
	}
}

func playKubeCmd(c *cliconfig.KubePlayValues) error {
	args := c.InputArgs
	if len(args) > 1 {
		return errors.New("you can only play one kubernetes file at a time")
	}
	if len(args) < 1 {
		return errors.New("you must supply at least one file")
	}

	if c.Authfile != "" {
		if _, err := os.Stat(c.Authfile); err != nil {
			return errors.Wrapf(err, "error getting authfile %s", c.Authfile)
		}
	}

	ctx := getContext()
	runtime, err := adapter.GetRuntime(ctx, &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.DeferredShutdown(false)

	_, err = runtime.PlayKubeYAML(ctx, c, args[0])
	return err
}
