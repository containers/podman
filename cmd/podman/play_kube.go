package main

import (
	"context"
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
	"github.com/containers/libpod/libpod/image"
	ns "github.com/containers/libpod/pkg/namespaces"
	"github.com/containers/libpod/pkg/spec"
	"github.com/containers/storage"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/api/core/v1"
)

const (
	// https://kubernetes.io/docs/concepts/storage/volumes/#hostpath
	createDirectoryPermission = 0755
	// https://kubernetes.io/docs/concepts/storage/volumes/#hostpath
	createFilePermission = 0644
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
			return playKubeYAMLCmd(&playKubeCommand)
		},
		Example: `podman play kube demo.yml
  podman play kube --cert-dir /mycertsdir --tls-verify=true --quiet myWebPod`,
	}
)

func init() {
	playKubeCommand.Command = _playKubeCommand
	playKubeCommand.SetHelpTemplate(HelpTemplate())
	playKubeCommand.SetUsageTemplate(UsageTemplate())
	flags := playKubeCommand.Flags()
	flags.StringVar(&playKubeCommand.Authfile, "authfile", "", "Path of the authentication file. Default is ${XDG_RUNTIME_DIR}/containers/auth.json. Use REGISTRY_AUTH_FILE environment variable to override")
	flags.StringVar(&playKubeCommand.CertDir, "cert-dir", "", "`Pathname` of a directory containing TLS certificates and keys")
	flags.StringVar(&playKubeCommand.Creds, "creds", "", "`Credentials` (USERNAME:PASSWORD) to use for authenticating to a registry")
	flags.BoolVarP(&playKubeCommand.Quiet, "quiet", "q", false, "Suppress output information when pulling images")
	flags.StringVar(&playKubeCommand.SignaturePolicy, "signature-policy", "", "`Pathname` of signature policy file (not usually used)")
	flags.BoolVar(&playKubeCommand.TlsVerify, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")
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

	// check for name collision between pod and container
	podName := podYAML.ObjectMeta.Name
	for _, n := range podYAML.Spec.Containers {
		if n.Name == podName {
			fmt.Printf("a container exists with the same name (%s) as the pod in your YAML file; changing pod name to %s_pod\n", podName, podName)
			podName = fmt.Sprintf("%s_pod", podName)
		}
	}

	podOptions = append(podOptions, libpod.WithInfraContainer())
	podOptions = append(podOptions, libpod.WithPodName(podName))
	// TODO for now we just used the default kernel namespaces; we need to add/subtract this from yaml

	nsOptions, err := shared.GetNamespaceOptions(strings.Split(shared.DefaultKernelNamespaces, ","))
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

	dockerRegistryOptions := image.DockerRegistryOptions{
		DockerRegistryCreds: registryCreds,
		DockerCertPath:      c.CertDir,
	}
	if c.Flag("tls-verify").Changed {
		dockerRegistryOptions.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!c.TlsVerify)
	}

	// map from name to mount point
	volumes := make(map[string]string)
	for _, volume := range podYAML.Spec.Volumes {
		hostPath := volume.VolumeSource.HostPath
		if hostPath == nil {
			return errors.Errorf("HostPath is currently the only supported VolumeSource")
		}
		if hostPath.Type != nil {
			switch *hostPath.Type {
			case v1.HostPathDirectoryOrCreate:
				if _, err := os.Stat(hostPath.Path); os.IsNotExist(err) {
					if err := os.Mkdir(hostPath.Path, createDirectoryPermission); err != nil {
						return errors.Errorf("Error creating HostPath %s at %s", volume.Name, hostPath.Path)
					}
				}
				// unconditionally label a newly created volume as private
				if err := libpod.LabelVolumePath(hostPath.Path, false); err != nil {
					return errors.Wrapf(err, "Error giving %s a label", hostPath.Path)
				}
				break
			case v1.HostPathFileOrCreate:
				if _, err := os.Stat(hostPath.Path); os.IsNotExist(err) {
					f, err := os.OpenFile(hostPath.Path, os.O_RDONLY|os.O_CREATE, createFilePermission)
					if err != nil {
						return errors.Errorf("Error creating HostPath %s at %s", volume.Name, hostPath.Path)
					}
					if err := f.Close(); err != nil {
						logrus.Warnf("Error in closing newly created HostPath file: %v", err)
					}
				}
				// unconditionally label a newly created volume as private
				if err := libpod.LabelVolumePath(hostPath.Path, false); err != nil {
					return errors.Wrapf(err, "Error giving %s a label", hostPath.Path)
				}
				break
			case v1.HostPathDirectory:
			case v1.HostPathFile:
			case v1.HostPathUnset:
				// do nothing here because we will verify the path exists in validateVolumeHostDir
				break
			default:
				return errors.Errorf("Directories are the only supported HostPath type")
			}
		}
		if err := shared.ValidateVolumeHostDir(hostPath.Path); err != nil {
			return errors.Wrapf(err, "Error in parsing HostPath in YAML")
		}
		volumes[volume.Name] = hostPath.Path
	}

	for _, container := range podYAML.Spec.Containers {
		newImage, err := runtime.ImageRuntime().New(ctx, container.Image, c.SignaturePolicy, c.Authfile, writer, &dockerRegistryOptions, image.SigningOptions{}, false, nil)
		if err != nil {
			return err
		}
		createConfig, err := kubeContainerToCreateConfig(ctx, container, runtime, newImage, namespaces, volumes)
		if err != nil {
			return err
		}
		ctr, err := shared.CreateContainerFromCreateConfig(runtime, createConfig, ctx, pod)
		if err != nil {
			return err
		}
		containers = append(containers, ctr)
	}

	// start the containers
	for _, ctr := range containers {
		if err := ctr.Start(ctx, true); err != nil {
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
func kubeContainerToCreateConfig(ctx context.Context, containerYAML v1.Container, runtime *libpod.Runtime, newImage *image.Image, namespaces map[string]string, volumes map[string]string) (*createconfig.CreateConfig, error) {
	var (
		containerConfig createconfig.CreateConfig
		envs            map[string]string
	)

	// The default for MemorySwappiness is -1, not 0
	containerConfig.Resources.MemorySwappiness = -1

	containerConfig.Runtime = runtime
	containerConfig.Image = containerYAML.Image
	containerConfig.ImageID = newImage.ID()
	containerConfig.Name = containerYAML.Name
	containerConfig.Tty = containerYAML.TTY
	containerConfig.WorkDir = containerYAML.WorkingDir

	imageData, _ := newImage.Inspect(ctx)

	containerConfig.User = "0"
	if imageData != nil {
		containerConfig.User = imageData.Config.User
	}

	if containerConfig.SecurityOpts != nil {
		if containerYAML.SecurityContext.ReadOnlyRootFilesystem != nil {
			containerConfig.ReadOnlyRootfs = *containerYAML.SecurityContext.ReadOnlyRootFilesystem
		}
		if containerYAML.SecurityContext.Privileged != nil {
			containerConfig.Privileged = *containerYAML.SecurityContext.Privileged
		}

		if containerYAML.SecurityContext.AllowPrivilegeEscalation != nil {
			containerConfig.NoNewPrivs = !*containerYAML.SecurityContext.AllowPrivilegeEscalation
		}
	}

	containerConfig.Command = []string{}
	if imageData != nil && imageData.Config != nil {
		containerConfig.Command = append(containerConfig.Command, imageData.Config.Entrypoint...)
	}
	if len(containerConfig.Command) != 0 {
		containerConfig.Command = append(containerConfig.Command, containerYAML.Command...)
	} else if imageData != nil && imageData.Config != nil {
		containerConfig.Command = append(containerConfig.Command, imageData.Config.Cmd...)
	}
	if imageData != nil && len(containerConfig.Command) == 0 {
		return nil, errors.Errorf("No command specified in container YAML or as CMD or ENTRYPOINT in this image for %s", containerConfig.Name)
	}

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
	if len(containerConfig.WorkDir) == 0 {
		containerConfig.WorkDir = "/"
	}
	if len(containerYAML.Env) > 0 {
		envs = make(map[string]string)
	}
	// Environment Variables
	for _, e := range containerYAML.Env {
		envs[e.Name] = e.Value
	}
	containerConfig.Env = envs

	for _, volume := range containerYAML.VolumeMounts {
		host_path, exists := volumes[volume.Name]
		if !exists {
			return nil, errors.Errorf("Volume mount %s specified for container but not configured in volumes", volume.Name)
		}
		if err := shared.ValidateVolumeCtrDir(volume.MountPath); err != nil {
			return nil, errors.Wrapf(err, "error in parsing MountPath")
		}
		containerConfig.Volumes = append(containerConfig.Volumes, fmt.Sprintf("%s:%s", host_path, volume.MountPath))
	}
	return &containerConfig, nil
}
