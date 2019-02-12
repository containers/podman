package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/containers/image/types"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	image2 "github.com/containers/libpod/libpod/image"
	ns "github.com/containers/libpod/pkg/namespaces"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/spec"
	"github.com/containers/storage"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/api/core/v1"
)

var (
	playKubeCommand     cliconfig.KubePlayValues
	playKubeDescription = "Play a Pod and its containers based on a Kubrernetes YAML"
	_playKubeCommand    = &cobra.Command{
		Use:   "kube",
		Short: "Play a pod based on Kubernetes YAML",
		Long:  playKubeDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			playKubeCommand.InputArgs = args
			playKubeCommand.GlobalFlags = MainGlobalOpts
			return playKubeYAMLCmd(&playKubeCommand)
		},
		Example: "Kubernetes YAML file",
	}
)

func init() {
	playKubeCommand.Command = _playKubeCommand
	playKubeCommand.SetUsageTemplate(UsageTemplate())
	flags := playKubeCommand.Flags()
	flags.StringVar(&playKubeCommand.Authfile, "authfile", "", "Path of the authentication file. Default is ${XDG_RUNTIME_DIR}/containers/auth.json. Use REGISTRY_AUTH_FILE environment variable to override")
	flags.StringVar(&playKubeCommand.CertDir, "cert-dir", "", "`Pathname` of a directory containing TLS certificates and keys")
	flags.StringVar(&playKubeCommand.Creds, "creds", "", "`Credentials` (USERNAME:PASSWORD) to use for authenticating to a registry")
	flags.BoolVarP(&playKubeCommand.Quiet, "quiet", "q", false, "Suppress output information when pulling images")
	flags.StringVar(&playKubeCommand.SignaturePolicy, "signature-policy", "", "`Pathname` of signature policy file (not usually used)")
	flags.BoolVar(&playKubeCommand.TlsVerify, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries (default: true)")

	rootCmd.AddCommand(playKubeCommand.Command)
}

func playKubeYAMLCmd(c *cliconfig.KubePlayValues) error {
	var (
		podOptions    []libpod.PodCreateOption
		podYAML       v1.Pod
		registryCreds *types.DockerAuthConfig
		containers    []*libpod.Container
		writer        io.Writer
	)

	ctx := getContext()
	if rootless.IsRootless() {
		return errors.Wrapf(libpod.ErrNotImplemented, "rootless users")
	}
	args := c.InputArgs
	if len(args) > 1 {
		return errors.New("you can only play one kubernetes file at a time")
	}
	if len(args) < 1 {
		return errors.New("you must supply at least one file")
	}

	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	content, err := ioutil.ReadFile(args[0])
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(content, &podYAML); err != nil {
		return errors.Wrapf(err, "unable to read %s as YAML", args[0])
	}

	podOptions = append(podOptions, libpod.WithInfraContainer())
	podOptions = append(podOptions, libpod.WithPodName(podYAML.ObjectMeta.Name))
	// TODO for now we just used the default kernel namespaces; we need to add/subtract this from yaml

	nsOptions, err := shared.GetNamespaceOptions(strings.Split(DefaultKernelNamespaces, ","))
	if err != nil {
		return err
	}
	podOptions = append(podOptions, nsOptions...)
	podPorts := getPodPorts(podYAML.Spec.Containers)
	podOptions = append(podOptions, libpod.WithInfraContainerPorts(podPorts))

	// Create the Pod
	pod, err := runtime.NewPod(ctx, podOptions...)
	if err != nil {
		return err
	}
	// Print the Pod's ID
	fmt.Println(pod.ID())

	podInfraID, err := pod.InfraContainerID()
	if err != nil {
		return err
	}

	namespaces := map[string]string{
		// Disabled during code review per mheon
		//"pid":  fmt.Sprintf("container:%s", podInfraID),
		"net":  fmt.Sprintf("container:%s", podInfraID),
		"user": fmt.Sprintf("container:%s", podInfraID),
		"ipc":  fmt.Sprintf("container:%s", podInfraID),
		"uts":  fmt.Sprintf("container:%s", podInfraID),
	}
	if !c.Quiet {
		writer = os.Stderr
	}

	dockerRegistryOptions := image2.DockerRegistryOptions{
		DockerRegistryCreds: registryCreds,
		DockerCertPath:      c.CertDir,
	}
	if c.Flag("tls-verify").Changed {
		dockerRegistryOptions.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!c.TlsVerify)
	}

	for _, container := range podYAML.Spec.Containers {
		newImage, err := runtime.ImageRuntime().New(ctx, container.Image, c.SignaturePolicy, c.Authfile, writer, &dockerRegistryOptions, image2.SigningOptions{}, false, nil)
		if err != nil {
			return err
		}
		createConfig := kubeContainerToCreateConfig(container, runtime, newImage, namespaces)
		if err != nil {
			return err
		}
		ctr, err := createContainerFromCreateConfig(runtime, createConfig, ctx, pod)
		if err != nil {
			return err
		}
		containers = append(containers, ctr)
	}

	// start the containers
	for _, ctr := range containers {
		if err := ctr.Start(ctx); err != nil {
			// Making this a hard failure here to avoid a mess
			// the other containers are in created status
			return err
		}
		fmt.Println(ctr.ID())
	}

	return nil
}

// getPodPorts converts a slice of kube container descriptions to an
// array of ocicni portmapping descriptions usable in libpod
func getPodPorts(containers []v1.Container) []ocicni.PortMapping {
	var infraPorts []ocicni.PortMapping
	for _, container := range containers {
		for _, p := range container.Ports {
			portBinding := ocicni.PortMapping{
				HostPort:      p.HostPort,
				ContainerPort: p.ContainerPort,
				Protocol:      strings.ToLower(string(p.Protocol)),
			}
			if p.HostIP != "" {
				logrus.Debug("HostIP on port bindings is not supported")
			}
			infraPorts = append(infraPorts, portBinding)
		}
	}
	return infraPorts
}

// kubeContainerToCreateConfig takes a v1.Container and returns a createconfig describing a container
func kubeContainerToCreateConfig(containerYAML v1.Container, runtime *libpod.Runtime, newImage *image2.Image, namespaces map[string]string) *createconfig.CreateConfig {
	var (
		containerConfig createconfig.CreateConfig
		envs            map[string]string
	)

	containerConfig.Runtime = runtime
	containerConfig.Image = containerYAML.Image
	containerConfig.ImageID = newImage.ID()
	containerConfig.Name = containerYAML.Name
	containerConfig.Tty = containerYAML.TTY
	containerConfig.WorkDir = containerYAML.WorkingDir
	if containerYAML.SecurityContext.ReadOnlyRootFilesystem != nil {
		containerConfig.ReadOnlyRootfs = *containerYAML.SecurityContext.ReadOnlyRootFilesystem
	}
	if containerYAML.SecurityContext.Privileged != nil {
		containerConfig.Privileged = *containerYAML.SecurityContext.Privileged
	}

	if containerYAML.SecurityContext.AllowPrivilegeEscalation != nil {
		containerConfig.NoNewPrivs = !*containerYAML.SecurityContext.AllowPrivilegeEscalation
	}

	containerConfig.Command = containerYAML.Command
	containerConfig.StopSignal = 15

	// If the user does not pass in ID mappings, just set to basics
	if containerConfig.IDMappings == nil {
		containerConfig.IDMappings = &storage.IDMappingOptions{}
	}

	containerConfig.NetMode = ns.NetworkMode(namespaces["net"])
	containerConfig.IpcMode = ns.IpcMode(namespaces["ipc"])
	containerConfig.UtsMode = ns.UTSMode(namespaces["uts"])
	// disabled in code review per mheon
	//containerConfig.PidMode = ns.PidMode(namespaces["pid"])
	containerConfig.UsernsMode = ns.UsernsMode(namespaces["user"])

	if len(containerYAML.Env) > 0 {
		envs = make(map[string]string)
	}
	// Environment Variables
	for _, e := range containerYAML.Env {
		envs[e.Name] = e.Value
	}
	containerConfig.Env = envs
	return &containerConfig
}
