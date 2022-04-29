package pods

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/common/pkg/completion"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/utils"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/errorhandling"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// playKubeOptionsWrapper allows for separating CLI-only fields from API-only
// fields.
type playKubeOptionsWrapper struct {
	entities.PlayKubeOptions

	TLSVerifyCLI   bool
	CredentialsCLI string
	StartCLI       bool
	BuildCLI       bool
}

var (
	annotations []string
	macs        []string
	// https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/
	defaultSeccompRoot = "/var/lib/kubelet/seccomp"
	kubeOptions        = playKubeOptionsWrapper{}
	kubeDescription    = `Command reads in a structured file of Kubernetes YAML.

  It creates pods or volumes based on the Kubernetes kind described in the YAML. Supported kinds are Pods, Deployments and PersistentVolumeClaims.`

	kubeCmd = &cobra.Command{
		Use:               "kube [options] KUBEFILE|-",
		Short:             "Play a pod or volume based on Kubernetes YAML.",
		Long:              kubeDescription,
		RunE:              kube,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompleteDefaultOneArg,
		Example: `podman play kube nginx.yml
  cat nginx.yml | podman play kube -
  podman play kube --creds user:password --seccomp-profile-root /custom/path apache.yml`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: kubeCmd,
		Parent:  playCmd,
	})

	flags := kubeCmd.Flags()
	flags.SetNormalizeFunc(utils.AliasFlags)

	annotationFlagName := "annotation"
	flags.StringSliceVar(
		&annotations,
		annotationFlagName, []string{},
		"Add annotations to pods (key=value)",
	)
	_ = kubeCmd.RegisterFlagCompletionFunc(annotationFlagName, completion.AutocompleteNone)
	credsFlagName := "creds"
	flags.StringVar(&kubeOptions.CredentialsCLI, credsFlagName, "", "`Credentials` (USERNAME:PASSWORD) to use for authenticating to a registry")
	_ = kubeCmd.RegisterFlagCompletionFunc(credsFlagName, completion.AutocompleteNone)

	staticMACFlagName := "mac-address"
	flags.StringSliceVar(&macs, staticMACFlagName, nil, "Static MAC addresses to assign to the pods")
	_ = kubeCmd.RegisterFlagCompletionFunc(staticMACFlagName, completion.AutocompleteNone)

	networkFlagName := "network"
	flags.StringArrayVar(&kubeOptions.Networks, networkFlagName, nil, "Connect pod to network(s) or network mode")
	_ = kubeCmd.RegisterFlagCompletionFunc(networkFlagName, common.AutocompleteNetworkFlag)

	staticIPFlagName := "ip"
	flags.IPSliceVar(&kubeOptions.StaticIPs, staticIPFlagName, nil, "Static IP addresses to assign to the pods")
	_ = kubeCmd.RegisterFlagCompletionFunc(staticIPFlagName, completion.AutocompleteNone)

	logDriverFlagName := "log-driver"
	flags.StringVar(&kubeOptions.LogDriver, logDriverFlagName, common.LogDriver(), "Logging driver for the container")
	_ = kubeCmd.RegisterFlagCompletionFunc(logDriverFlagName, common.AutocompleteLogDriver)

	logOptFlagName := "log-opt"
	flags.StringSliceVar(
		&kubeOptions.LogOptions,
		logOptFlagName, []string{},
		"Logging driver options",
	)
	_ = kubeCmd.RegisterFlagCompletionFunc(logOptFlagName, common.AutocompleteLogOpt)

	flags.BoolVar(&kubeOptions.NoHosts, "no-hosts", false, "Do not create /etc/hosts within the pod's containers, instead use the version from the image")
	flags.BoolVarP(&kubeOptions.Quiet, "quiet", "q", false, "Suppress output information when pulling images")
	flags.BoolVar(&kubeOptions.TLSVerifyCLI, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")
	flags.BoolVar(&kubeOptions.StartCLI, "start", true, "Start the pod after creating it")

	authfileFlagName := "authfile"
	flags.StringVar(&kubeOptions.Authfile, authfileFlagName, auth.GetDefaultAuthFile(), "Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	_ = kubeCmd.RegisterFlagCompletionFunc(authfileFlagName, completion.AutocompleteDefault)

	downFlagName := "down"
	flags.BoolVar(&kubeOptions.Down, downFlagName, false, "Stop pods defined in the YAML file")

	replaceFlagName := "replace"
	flags.BoolVar(&kubeOptions.Replace, replaceFlagName, false, "Delete and recreate pods defined in the YAML file")

	if !registry.IsRemote() {
		certDirFlagName := "cert-dir"
		flags.StringVar(&kubeOptions.CertDir, certDirFlagName, "", "`Pathname` of a directory containing TLS certificates and keys")
		_ = kubeCmd.RegisterFlagCompletionFunc(certDirFlagName, completion.AutocompleteDefault)

		seccompProfileRootFlagName := "seccomp-profile-root"
		flags.StringVar(&kubeOptions.SeccompProfileRoot, seccompProfileRootFlagName, defaultSeccompRoot, "Directory path for seccomp profiles")
		_ = kubeCmd.RegisterFlagCompletionFunc(seccompProfileRootFlagName, completion.AutocompleteDefault)

		configmapFlagName := "configmap"
		flags.StringSliceVar(&kubeOptions.ConfigMaps, configmapFlagName, []string{}, "`Pathname` of a YAML file containing a kubernetes configmap")
		_ = kubeCmd.RegisterFlagCompletionFunc(configmapFlagName, completion.AutocompleteDefault)

		buildFlagName := "build"
		flags.BoolVar(&kubeOptions.BuildCLI, buildFlagName, false, "Build all images in a YAML (given Containerfiles exist)")

		contextDirFlagName := "context-dir"
		flags.StringVar(&kubeOptions.ContextDir, contextDirFlagName, "", "Path to top level of context directory")
		_ = kubeCmd.RegisterFlagCompletionFunc(contextDirFlagName, completion.AutocompleteDefault)

		flags.StringVar(&kubeOptions.SignaturePolicy, "signature-policy", "", "`Pathname` of signature policy file (not usually used)")

		_ = flags.MarkHidden("signature-policy")
	}
}

func kube(cmd *cobra.Command, args []string) error {
	// TLS verification in c/image is controlled via a `types.OptionalBool`
	// which allows for distinguishing among set-true, set-false, unspecified
	// which is important to implement a sane way of dealing with defaults of
	// boolean CLI flags.
	if cmd.Flags().Changed("tls-verify") {
		kubeOptions.SkipTLSVerify = types.NewOptionalBool(!kubeOptions.TLSVerifyCLI)
	}
	if cmd.Flags().Changed("start") {
		kubeOptions.Start = types.NewOptionalBool(kubeOptions.StartCLI)
	}
	if cmd.Flags().Changed("build") {
		kubeOptions.Build = types.NewOptionalBool(kubeOptions.BuildCLI)
	}
	if kubeOptions.Authfile != "" {
		if _, err := os.Stat(kubeOptions.Authfile); err != nil {
			return err
		}
	}
	if kubeOptions.ContextDir != "" && kubeOptions.Build != types.OptionalBoolTrue {
		return errors.New("--build must be specified when using --context-dir option")
	}
	if kubeOptions.CredentialsCLI != "" {
		creds, err := util.ParseRegistryCreds(kubeOptions.CredentialsCLI)
		if err != nil {
			return err
		}
		kubeOptions.Username = creds.Username
		kubeOptions.Password = creds.Password
	}

	for _, annotation := range annotations {
		splitN := strings.SplitN(annotation, "=", 2)
		if len(splitN) > 2 {
			return errors.Errorf("annotation %q must include an '=' sign", annotation)
		}
		if kubeOptions.Annotations == nil {
			kubeOptions.Annotations = make(map[string]string)
		}
		annotation := splitN[1]
		if len(annotation) > define.MaxKubeAnnotation {
			return errors.Errorf("annotation exceeds maximum size, %d, of kubernetes annotation: %s", define.MaxKubeAnnotation, annotation)
		}
		kubeOptions.Annotations[splitN[0]] = annotation
	}
	yamlfile := args[0]
	if yamlfile == "-" {
		yamlfile = "/dev/stdin"
	}

	for _, mac := range macs {
		m, err := net.ParseMAC(mac)
		if err != nil {
			return err
		}
		kubeOptions.StaticMACs = append(kubeOptions.StaticMACs, m)
	}
	if kubeOptions.Down {
		return teardown(yamlfile)
	}
	if kubeOptions.Replace {
		if err := teardown(yamlfile); err != nil && !errorhandling.Contains(err, define.ErrNoSuchPod) {
			return err
		}
	}
	return playkube(yamlfile)
}

func teardown(yamlfile string) error {
	var (
		podStopErrors utils.OutputErrors
		podRmErrors   utils.OutputErrors
	)
	options := new(entities.PlayKubeDownOptions)
	f, err := os.Open(yamlfile)
	if err != nil {
		return err
	}
	defer f.Close()
	reports, err := registry.ContainerEngine().PlayKubeDown(registry.GetContext(), f, *options)
	if err != nil {
		return errors.Wrap(err, yamlfile)
	}

	// Output stopped pods
	fmt.Println("Pods stopped:")
	for _, stopped := range reports.StopReport {
		if len(stopped.Errs) == 0 {
			fmt.Println(stopped.Id)
		} else {
			podStopErrors = append(podStopErrors, stopped.Errs...)
		}
	}
	// Dump any stop errors
	lastStopError := podStopErrors.PrintErrors()
	if lastStopError != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", lastStopError)
	}

	// Output rm'd pods
	fmt.Println("Pods removed:")
	for _, removed := range reports.RmReport {
		if removed.Err == nil {
			fmt.Println(removed.Id)
		} else {
			podRmErrors = append(podRmErrors, removed.Err)
		}
	}
	return podRmErrors.PrintErrors()
}

func playkube(yamlfile string) error {
	f, err := os.Open(yamlfile)
	if err != nil {
		return err
	}
	defer f.Close()
	report, err := registry.ContainerEngine().PlayKube(registry.GetContext(), f, kubeOptions.PlayKubeOptions)
	if err != nil {
		return errors.Wrap(err, yamlfile)
	}
	// Print volumes report
	for i, volume := range report.Volumes {
		if i == 0 {
			fmt.Println("Volumes:")
		}
		fmt.Println(volume.Name)
	}

	// Print pods report
	for _, pod := range report.Pods {
		for _, l := range pod.Logs {
			fmt.Fprint(os.Stderr, l)
		}
	}

	ctrsFailed := 0

	for _, pod := range report.Pods {
		fmt.Println("Pod:")
		fmt.Println(pod.ID)

		switch len(pod.Containers) {
		case 0:
			continue
		case 1:
			fmt.Println("Container:")
		default:
			fmt.Println("Containers:")
		}
		for _, ctr := range pod.Containers {
			fmt.Println(ctr)
		}
		ctrsFailed += len(pod.ContainerErrors)
		// If We have errors, add a newline
		if len(pod.ContainerErrors) > 0 {
			fmt.Println()
		}
		for _, err := range pod.ContainerErrors {
			fmt.Fprintln(os.Stderr, err)
		}
		// Empty line for space for next block
		fmt.Println()
	}

	if ctrsFailed > 0 {
		return errors.Errorf("failed to start %d containers", ctrsFailed)
	}

	return nil
}
