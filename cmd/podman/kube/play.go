package kube

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/common/pkg/completion"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/parse"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/utils"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/errorhandling"
	"github.com/containers/podman/v4/pkg/util"
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
	annotations    []string
	macs           []string
}

var (
	// https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/
	defaultSeccompRoot = "/var/lib/kubelet/seccomp"
	playOptions        = playKubeOptionsWrapper{}
	playDescription    = `Reads in a structured file of Kubernetes YAML.

  Creates pods or volumes based on the Kubernetes kind described in the YAML. Supported kinds are Pods, Deployments and PersistentVolumeClaims.`

	playCmd = &cobra.Command{
		Use:               "play [options] KUBEFILE|-",
		Short:             "Play a pod or volume based on Kubernetes YAML.",
		Long:              playDescription,
		RunE:              play,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompleteDefaultOneArg,
		Example: `podman kube play nginx.yml
  cat nginx.yml | podman kube play -
  podman kube play --creds user:password --seccomp-profile-root /custom/path apache.yml
  podman kube play https://example.com/nginx.yml`,
	}
)

var (
	playKubeCmd = &cobra.Command{
		Use:               "kube [options] KUBEFILE|-",
		Short:             "Play a pod or volume based on Kubernetes YAML.",
		Long:              playDescription,
		Hidden:            true,
		RunE:              playKube,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompleteDefaultOneArg,
		Example: `podman play kube nginx.yml
  cat nginx.yml | podman play kube -
  podman play kube --creds user:password --seccomp-profile-root /custom/path apache.yml
  podman play kube https://example.com/nginx.yml`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: playCmd,
		Parent:  kubeCmd,
	})
	playFlags(playCmd)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: playKubeCmd,
		Parent:  playKubeParentCmd,
	})
	playFlags(playKubeCmd)
}

func playFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.SetNormalizeFunc(utils.AliasFlags)

	annotationFlagName := "annotation"
	flags.StringSliceVar(
		&playOptions.annotations,
		annotationFlagName, []string{},
		"Add annotations to pods (key=value)",
	)
	_ = cmd.RegisterFlagCompletionFunc(annotationFlagName, completion.AutocompleteNone)
	credsFlagName := "creds"
	flags.StringVar(&playOptions.CredentialsCLI, credsFlagName, "", "`Credentials` (USERNAME:PASSWORD) to use for authenticating to a registry")
	_ = cmd.RegisterFlagCompletionFunc(credsFlagName, completion.AutocompleteNone)

	staticMACFlagName := "mac-address"
	flags.StringSliceVar(&playOptions.macs, staticMACFlagName, nil, "Static MAC addresses to assign to the pods")
	_ = cmd.RegisterFlagCompletionFunc(staticMACFlagName, completion.AutocompleteNone)

	networkFlagName := "network"
	flags.StringArrayVar(&playOptions.Networks, networkFlagName, nil, "Connect pod to network(s) or network mode")
	_ = cmd.RegisterFlagCompletionFunc(networkFlagName, common.AutocompleteNetworkFlag)

	staticIPFlagName := "ip"
	flags.IPSliceVar(&playOptions.StaticIPs, staticIPFlagName, nil, "Static IP addresses to assign to the pods")
	_ = cmd.RegisterFlagCompletionFunc(staticIPFlagName, completion.AutocompleteNone)

	logDriverFlagName := "log-driver"
	flags.StringVar(&playOptions.LogDriver, logDriverFlagName, common.LogDriver(), "Logging driver for the container")
	_ = cmd.RegisterFlagCompletionFunc(logDriverFlagName, common.AutocompleteLogDriver)

	logOptFlagName := "log-opt"
	flags.StringSliceVar(
		&playOptions.LogOptions,
		logOptFlagName, []string{},
		"Logging driver options",
	)
	_ = cmd.RegisterFlagCompletionFunc(logOptFlagName, common.AutocompleteLogOpt)

	usernsFlagName := "userns"
	flags.StringVar(&playOptions.Userns, usernsFlagName, os.Getenv("PODMAN_USERNS"),
		"User namespace to use",
	)
	_ = cmd.RegisterFlagCompletionFunc(usernsFlagName, common.AutocompleteUserNamespace)

	flags.BoolVar(&playOptions.NoHosts, "no-hosts", false, "Do not create /etc/hosts within the pod's containers, instead use the version from the image")
	flags.BoolVarP(&playOptions.Quiet, "quiet", "q", false, "Suppress output information when pulling images")
	flags.BoolVar(&playOptions.TLSVerifyCLI, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")
	flags.BoolVar(&playOptions.StartCLI, "start", true, "Start the pod after creating it")

	authfileFlagName := "authfile"
	flags.StringVar(&playOptions.Authfile, authfileFlagName, auth.GetDefaultAuthFile(), "Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	_ = cmd.RegisterFlagCompletionFunc(authfileFlagName, completion.AutocompleteDefault)

	downFlagName := "down"
	flags.BoolVar(&playOptions.Down, downFlagName, false, "Stop pods defined in the YAML file")
	_ = flags.MarkHidden("down")

	replaceFlagName := "replace"
	flags.BoolVar(&playOptions.Replace, replaceFlagName, false, "Delete and recreate pods defined in the YAML file")

	if !registry.IsRemote() {
		certDirFlagName := "cert-dir"
		flags.StringVar(&playOptions.CertDir, certDirFlagName, "", "`Pathname` of a directory containing TLS certificates and keys")
		_ = cmd.RegisterFlagCompletionFunc(certDirFlagName, completion.AutocompleteDefault)

		seccompProfileRootFlagName := "seccomp-profile-root"
		flags.StringVar(&playOptions.SeccompProfileRoot, seccompProfileRootFlagName, defaultSeccompRoot, "Directory path for seccomp profiles")
		_ = cmd.RegisterFlagCompletionFunc(seccompProfileRootFlagName, completion.AutocompleteDefault)

		configmapFlagName := "configmap"
		flags.StringSliceVar(&playOptions.ConfigMaps, configmapFlagName, []string{}, "`Pathname` of a YAML file containing a kubernetes configmap")
		_ = cmd.RegisterFlagCompletionFunc(configmapFlagName, completion.AutocompleteDefault)

		buildFlagName := "build"
		flags.BoolVar(&playOptions.BuildCLI, buildFlagName, false, "Build all images in a YAML (given Containerfiles exist)")

		contextDirFlagName := "context-dir"
		flags.StringVar(&playOptions.ContextDir, contextDirFlagName, "", "Path to top level of context directory")
		_ = cmd.RegisterFlagCompletionFunc(contextDirFlagName, completion.AutocompleteDefault)

		// NOTE: The service-container flag is marked as hidden as it
		// is purely designed for running kube-play or play-kube in systemd units.
		// It is not something users should need to know or care about.
		//
		// Having a flag rather than an env variable is cleaner.
		serviceFlagName := "service-container"
		flags.BoolVar(&playOptions.ServiceContainer, serviceFlagName, false, "Starts a service container before all pods")
		_ = flags.MarkHidden("service-container")

		flags.StringVar(&playOptions.SignaturePolicy, "signature-policy", "", "`Pathname` of signature policy file (not usually used)")

		_ = flags.MarkHidden("signature-policy")
	}
}

func play(cmd *cobra.Command, args []string) error {
	if playOptions.ServiceContainer && !playOptions.StartCLI { // Sanity check to be future proof
		return fmt.Errorf("--service-container does not work with --start=stop")
	}
	// TLS verification in c/image is controlled via a `types.OptionalBool`
	// which allows for distinguishing among set-true, set-false, unspecified
	// which is important to implement a sane way of dealing with defaults of
	// boolean CLI flags.
	if cmd.Flags().Changed("tls-verify") {
		playOptions.SkipTLSVerify = types.NewOptionalBool(!playOptions.TLSVerifyCLI)
	}
	if cmd.Flags().Changed("start") {
		playOptions.Start = types.NewOptionalBool(playOptions.StartCLI)
	}
	if cmd.Flags().Changed("build") {
		playOptions.Build = types.NewOptionalBool(playOptions.BuildCLI)
	}
	if playOptions.Authfile != "" {
		if _, err := os.Stat(playOptions.Authfile); err != nil {
			return err
		}
	}
	if playOptions.ContextDir != "" && playOptions.Build != types.OptionalBoolTrue {
		return errors.New("--build must be specified when using --context-dir option")
	}
	if playOptions.CredentialsCLI != "" {
		creds, err := util.ParseRegistryCreds(playOptions.CredentialsCLI)
		if err != nil {
			return err
		}
		playOptions.Username = creds.Username
		playOptions.Password = creds.Password
	}

	for _, annotation := range playOptions.annotations {
		splitN := strings.SplitN(annotation, "=", 2)
		if len(splitN) > 2 {
			return fmt.Errorf("annotation %q must include an '=' sign", annotation)
		}
		if playOptions.Annotations == nil {
			playOptions.Annotations = make(map[string]string)
		}
		annotation := splitN[1]
		if len(annotation) > define.MaxKubeAnnotation {
			return fmt.Errorf("annotation exceeds maximum size, %d, of kubernetes annotation: %s", define.MaxKubeAnnotation, annotation)
		}
		playOptions.Annotations[splitN[0]] = annotation
	}

	for _, mac := range playOptions.macs {
		m, err := net.ParseMAC(mac)
		if err != nil {
			return err
		}
		playOptions.StaticMACs = append(playOptions.StaticMACs, m)
	}

	reader, err := readerFromArg(args[0])
	if err != nil {
		return err
	}

	if playOptions.Down {
		return teardown(reader)
	}

	if playOptions.Replace {
		if err := teardown(reader); err != nil && !errorhandling.Contains(err, define.ErrNoSuchPod) {
			return err
		}
		if _, err := reader.Seek(0, 0); err != nil {
			return err
		}
	}

	return kubeplay(reader)
}

func playKube(cmd *cobra.Command, args []string) error {
	return play(cmd, args)
}

func readerFromArg(fileName string) (*bytes.Reader, error) {
	errURL := parse.ValidURL(fileName)
	if fileName == "-" { // Read from stdin
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(data), nil
	}
	if errURL == nil {
		response, err := http.Get(fileName)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()

		data, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(data), nil
	}
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func teardown(body io.Reader) error {
	var (
		podStopErrors utils.OutputErrors
		podRmErrors   utils.OutputErrors
	)
	options := new(entities.PlayKubeDownOptions)
	reports, err := registry.ContainerEngine().PlayKubeDown(registry.GetContext(), body, *options)
	if err != nil {
		return err
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

func kubeplay(body io.Reader) error {
	report, err := registry.ContainerEngine().PlayKube(registry.GetContext(), body, playOptions.PlayKubeOptions)
	if err != nil {
		return err
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
		return fmt.Errorf("failed to start %d containers", ctrsFailed)
	}

	return nil
}
