package pods

import (
	"fmt"
	"os"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/cmd/podman/utils"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// playKubeOptionsWrapper allows for separating CLI-only fields from API-only
// fields.
type playKubeOptionsWrapper struct {
	entities.PlayKubeOptions

	TLSVerifyCLI   bool
	CredentialsCLI string
}

var (
	// https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/
	defaultSeccompRoot = "/var/lib/kubelet/seccomp"
	kubeOptions        = playKubeOptionsWrapper{}
	kubeDescription    = `Command reads in a structured file of Kubernetes YAML.

  It creates the pod and containers described in the YAML.  The containers within the pod are then started and the ID of the new Pod is output.`

	kubeCmd = &cobra.Command{
		Use:   "kube [flags] KUBEFILE",
		Short: "Play a pod based on Kubernetes YAML.",
		Long:  kubeDescription,
		RunE:  kube,
		Args:  cobra.ExactArgs(1),
		Example: `podman play kube nginx.yml
  podman play kube --creds user:password --seccomp-profile-root /custom/path apache.yml`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: kubeCmd,
		Parent:  playCmd,
	})

	flags := kubeCmd.Flags()
	flags.SetNormalizeFunc(utils.AliasFlags)
	flags.StringVar(&kubeOptions.CredentialsCLI, "creds", "", "`Credentials` (USERNAME:PASSWORD) to use for authenticating to a registry")
	flags.StringVar(&kubeOptions.Network, "network", "", "Connect pod to CNI network(s)")
	flags.BoolVarP(&kubeOptions.Quiet, "quiet", "q", false, "Suppress output information when pulling images")
	if !registry.IsRemote() {
		flags.StringVar(&kubeOptions.Authfile, "authfile", auth.GetDefaultAuthFile(), "Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
		flags.StringVar(&kubeOptions.CertDir, "cert-dir", "", "`Pathname` of a directory containing TLS certificates and keys")
		flags.BoolVar(&kubeOptions.TLSVerifyCLI, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")
		flags.StringVar(&kubeOptions.SignaturePolicy, "signature-policy", "", "`Pathname` of signature policy file (not usually used)")
		flags.StringVar(&kubeOptions.SeccompProfileRoot, "seccomp-profile-root", defaultSeccompRoot, "Directory path for seccomp profiles")
	}

	_ = flags.MarkHidden("signature-policy")
}

func kube(cmd *cobra.Command, args []string) error {
	// TLS verification in c/image is controlled via a `types.OptionalBool`
	// which allows for distinguishing among set-true, set-false, unspecified
	// which is important to implement a sane way of dealing with defaults of
	// boolean CLI flags.
	if cmd.Flags().Changed("tls-verify") {
		kubeOptions.SkipTLSVerify = types.NewOptionalBool(!kubeOptions.TLSVerifyCLI)
	}
	if kubeOptions.Authfile != "" {
		if _, err := os.Stat(kubeOptions.Authfile); err != nil {
			return errors.Wrapf(err, "error getting authfile %s", kubeOptions.Authfile)
		}
	}
	if kubeOptions.CredentialsCLI != "" {
		creds, err := util.ParseRegistryCreds(kubeOptions.CredentialsCLI)
		if err != nil {
			return err
		}
		kubeOptions.Username = creds.Username
		kubeOptions.Password = creds.Password
	}

	report, err := registry.ContainerEngine().PlayKube(registry.GetContext(), args[0], kubeOptions.PlayKubeOptions)
	if err != nil {
		return err
	}

	for _, l := range report.Logs {
		fmt.Fprintf(os.Stderr, l)
	}

	fmt.Printf("Pod:\n%s\n", report.Pod)
	switch len(report.Containers) {
	case 0:
		return nil
	case 1:
		fmt.Printf("Container:\n")
	default:
		fmt.Printf("Containers:\n")
	}
	for _, ctr := range report.Containers {
		fmt.Println(ctr)
	}

	return nil
}
