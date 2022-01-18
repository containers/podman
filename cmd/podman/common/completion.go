package common

import (
	"bufio"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/config"
	"github.com/containers/image/v5/pkg/sysregistriesv2"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/rootless"
	systemdDefine "github.com/containers/podman/v4/pkg/systemd/define"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/spf13/cobra"
)

var (
	// ChangeCmds is the list of valid Change commands to passed to the Commit call
	ChangeCmds = []string{"CMD", "ENTRYPOINT", "ENV", "EXPOSE", "LABEL", "ONBUILD", "STOPSIGNAL", "USER", "VOLUME", "WORKDIR"}
	// LogLevels supported by podman
	LogLevels = []string{"trace", "debug", "info", "warn", "warning", "error", "fatal", "panic"}
)

type completeType int

const (
	// complete names and IDs after two chars
	completeDefault completeType = iota
	// only complete IDs
	completeIDs
	// only complete Names
	completeNames
)

type keyValueCompletion map[string]func(s string) ([]string, cobra.ShellCompDirective)

func setupContainerEngine(cmd *cobra.Command) (entities.ContainerEngine, error) {
	containerEngine, err := registry.NewContainerEngine(cmd, []string{})
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, err
	}
	if !registry.IsRemote() && rootless.IsRootless() {
		_, noMoveProcess := cmd.Annotations[registry.NoMoveProcess]

		err := containerEngine.SetupRootless(registry.Context(), noMoveProcess)
		if err != nil {
			return nil, err
		}
	}
	return containerEngine, nil
}

func setupImageEngine(cmd *cobra.Command) (entities.ImageEngine, error) {
	imageEngine, err := registry.NewImageEngine(cmd, []string{})
	if err != nil {
		return nil, err
	}
	// we also need to set up the container engine since this
	// is required to setup the rootless namespace
	if _, err = setupContainerEngine(cmd); err != nil {
		return nil, err
	}
	return imageEngine, nil
}

func getContainers(cmd *cobra.Command, toComplete string, cType completeType, statuses ...string) ([]string, cobra.ShellCompDirective) {
	suggestions := []string{}
	listOpts := entities.ContainerListOptions{
		Filters: make(map[string][]string),
	}
	listOpts.All = true
	listOpts.Pod = true
	if len(statuses) > 0 {
		listOpts.Filters["status"] = statuses
	}

	engine, err := setupContainerEngine(cmd)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	containers, err := engine.ContainerList(registry.GetContext(), listOpts)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, c := range containers {
		// include ids in suggestions if cType == completeIDs or
		// more then 2 chars are typed and cType == completeDefault
		if ((len(toComplete) > 1 && cType == completeDefault) ||
			cType == completeIDs) && strings.HasPrefix(c.ID, toComplete) {
			suggestions = append(suggestions, c.ID[0:12]+"\t"+c.PodName)
		}
		// include name in suggestions
		if cType != completeIDs && strings.HasPrefix(c.Names[0], toComplete) {
			suggestions = append(suggestions, c.Names[0]+"\t"+c.PodName)
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func getPods(cmd *cobra.Command, toComplete string, cType completeType, statuses ...string) ([]string, cobra.ShellCompDirective) {
	suggestions := []string{}
	listOpts := entities.PodPSOptions{
		Filters: make(map[string][]string),
	}
	if len(statuses) > 0 {
		listOpts.Filters["status"] = statuses
	}

	engine, err := setupContainerEngine(cmd)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	pods, err := engine.PodPs(registry.GetContext(), listOpts)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, pod := range pods {
		// include ids in suggestions if cType == completeIDs or
		// more then 2 chars are typed and cType == completeDefault
		if ((len(toComplete) > 1 && cType == completeDefault) ||
			cType == completeIDs) && strings.HasPrefix(pod.Id, toComplete) {
			suggestions = append(suggestions, pod.Id[0:12])
		}
		// include name in suggestions
		if cType != completeIDs && strings.HasPrefix(pod.Name, toComplete) {
			suggestions = append(suggestions, pod.Name)
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func getVolumes(cmd *cobra.Command, toComplete string) ([]string, cobra.ShellCompDirective) {
	suggestions := []string{}
	lsOpts := entities.VolumeListOptions{}

	engine, err := setupContainerEngine(cmd)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	volumes, err := engine.VolumeList(registry.GetContext(), lsOpts)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, v := range volumes {
		if strings.HasPrefix(v.Name, toComplete) {
			suggestions = append(suggestions, v.Name)
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func getImages(cmd *cobra.Command, toComplete string) ([]string, cobra.ShellCompDirective) {
	suggestions := []string{}
	listOptions := entities.ImageListOptions{}

	engine, err := setupImageEngine(cmd)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	images, err := engine.List(registry.GetContext(), listOptions)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, image := range images {
		// include ids in suggestions if more then 2 chars are typed
		if len(toComplete) > 1 && strings.HasPrefix(image.ID, toComplete) {
			suggestions = append(suggestions, image.ID[0:12])
		}
		for _, repo := range image.RepoTags {
			if toComplete == "" {
				// suggest only full repo path if no input is given
				if strings.HasPrefix(repo, toComplete) {
					suggestions = append(suggestions, repo)
				}
			} else {
				// suggested "registry.fedoraproject.org/f29/httpd:latest" as
				// - "registry.fedoraproject.org/f29/httpd:latest"
				// - "f29/httpd:latest"
				// - "httpd:latest"
				paths := strings.Split(repo, "/")
				for i := range paths {
					suggestionWithTag := strings.Join(paths[i:], "/")
					if strings.HasPrefix(suggestionWithTag, toComplete) {
						suggestions = append(suggestions, suggestionWithTag)
					}
				}
			}
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func getSecrets(cmd *cobra.Command, toComplete string, cType completeType) ([]string, cobra.ShellCompDirective) {
	suggestions := []string{}

	engine, err := setupContainerEngine(cmd)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	secrets, err := engine.SecretList(registry.GetContext(), entities.SecretListRequest{})
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, s := range secrets {
		// works the same as in getNetworks
		if ((len(toComplete) > 1 && cType == completeDefault) ||
			cType == completeIDs) && strings.HasPrefix(s.ID, toComplete) {
			suggestions = append(suggestions, s.ID[0:12])
		}
		// include name in suggestions
		if cType != completeIDs && strings.HasPrefix(s.Spec.Name, toComplete) {
			suggestions = append(suggestions, s.Spec.Name)
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func getRegistries() ([]string, cobra.ShellCompDirective) {
	regs, err := sysregistriesv2.UnqualifiedSearchRegistries(nil)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return regs, cobra.ShellCompDirectiveNoFileComp
}

func getNetworks(cmd *cobra.Command, toComplete string, cType completeType) ([]string, cobra.ShellCompDirective) {
	suggestions := []string{}
	networkListOptions := entities.NetworkListOptions{}

	engine, err := setupContainerEngine(cmd)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	networks, err := engine.NetworkList(registry.Context(), networkListOptions)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, n := range networks {
		// include ids in suggestions if cType == completeIDs or
		// more then 2 chars are typed and cType == completeDefault
		if ((len(toComplete) > 1 && cType == completeDefault) ||
			cType == completeIDs) && strings.HasPrefix(n.ID, toComplete) {
			suggestions = append(suggestions, n.ID[0:12])
		}
		// include name in suggestions
		if cType != completeIDs && strings.HasPrefix(n.Name, toComplete) {
			suggestions = append(suggestions, n.Name)
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

// validCurrentCmdLine validates the current cmd line
// It utilizes the Args function from the cmd struct
// In most cases the Args function validates the args length but it
// is also used to verify that --latest is not given with an argument.
// This function helps to makes sure we only complete valid arguments.
func validCurrentCmdLine(cmd *cobra.Command, args []string, toComplete string) bool {
	if cmd.Args == nil {
		// Without an Args function we cannot check so assume it's correct
		return true
	}
	// We have to append toComplete to the args otherwise the
	// argument count would not match the expected behavior
	if err := cmd.Args(cmd, append(args, toComplete)); err != nil {
		// Special case if we use ExactArgs(2) or MinimumNArgs(2),
		// They will error if we try to complete the first arg.
		// Lets try to parse the common error and compare if we have less args than
		// required. In this case we are fine and should provide completion.

		// Clean the err msg so we can parse it with fmt.Sscanf
		// Trim MinimumNArgs prefix
		cleanErr := strings.TrimPrefix(err.Error(), "requires at least ")
		// Trim MinimumNArgs "only" part
		cleanErr = strings.ReplaceAll(cleanErr, "only received", "received")
		// Trim ExactArgs prefix
		cleanErr = strings.TrimPrefix(cleanErr, "accepts ")
		var need, got int
		cobra.CompDebugln(cleanErr, true)
		_, err = fmt.Sscanf(cleanErr, "%d arg(s), received %d", &need, &got)
		if err == nil {
			if need >= got {
				// We still need more arguments so provide more completions
				return true
			}
		}
		return false
	}
	return true
}

func prefixSlice(pre string, slice []string) []string {
	for i := range slice {
		slice[i] = pre + slice[i]
	}
	return slice
}

func suffixCompSlice(suf string, slice []string) []string {
	for i := range slice {
		split := strings.SplitN(slice[i], "\t", 2)
		if len(split) > 1 {
			slice[i] = split[0] + suf + "\t" + split[1]
		} else {
			slice[i] = slice[i] + suf
		}
	}
	return slice
}

func completeKeyValues(toComplete string, k keyValueCompletion) ([]string, cobra.ShellCompDirective) {
	suggestions := make([]string, 0, len(k))
	directive := cobra.ShellCompDirectiveNoFileComp
	for key, getComps := range k {
		if strings.HasPrefix(toComplete, key) {
			if getComps != nil {
				suggestions, dir := getComps(toComplete[len(key):])
				return prefixSlice(key, suggestions), dir
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		if strings.HasPrefix(key, toComplete) {
			suggestions = append(suggestions, key)
			latKey := key[len(key)-1:]
			if latKey == "=" || latKey == ":" {
				// make sure we don't add a space after ':' or '='
				directive = cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
			}
		}
	}
	return suggestions, directive
}

func getBoolCompletion(_ string) ([]string, cobra.ShellCompDirective) {
	return []string{"true", "false"}, cobra.ShellCompDirectiveNoFileComp
}

/* Autocomplete Functions for cobra ValidArgsFunction */

// AutocompleteContainers - Autocomplete all container names.
func AutocompleteContainers(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getContainers(cmd, toComplete, completeDefault)
}

// AutocompleteContainersCreated - Autocomplete only created container names.
func AutocompleteContainersCreated(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getContainers(cmd, toComplete, completeDefault, "created")
}

// AutocompleteContainersExited - Autocomplete only exited container names.
func AutocompleteContainersExited(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getContainers(cmd, toComplete, completeDefault, "exited")
}

// AutocompleteContainersPaused - Autocomplete only paused container names.
func AutocompleteContainersPaused(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getContainers(cmd, toComplete, completeDefault, "paused")
}

// AutocompleteContainersRunning - Autocomplete only running container names.
func AutocompleteContainersRunning(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getContainers(cmd, toComplete, completeDefault, "running")
}

// AutocompleteContainersStartable - Autocomplete only created and exited container names.
func AutocompleteContainersStartable(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getContainers(cmd, toComplete, completeDefault, "created", "exited")
}

// AutocompletePods - Autocomplete all pod names.
func AutocompletePods(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getPods(cmd, toComplete, completeDefault)
}

// AutocompletePodsRunning - Autocomplete only running pod names.
// It considers degraded as running.
func AutocompletePodsRunning(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getPods(cmd, toComplete, completeDefault, "running", "degraded")
}

// AutocompleteForKube - Autocomplete all Podman objects supported by kube generate.
func AutocompleteForKube(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	containers, _ := getContainers(cmd, toComplete, completeDefault)
	pods, _ := getPods(cmd, toComplete, completeDefault)
	volumes, _ := getVolumes(cmd, toComplete)
	objs := containers
	objs = append(objs, pods...)
	objs = append(objs, volumes...)
	return objs, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteContainersAndPods - Autocomplete container names and pod names.
func AutocompleteContainersAndPods(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	containers, _ := getContainers(cmd, toComplete, completeDefault)
	pods, _ := getPods(cmd, toComplete, completeDefault)
	return append(containers, pods...), cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteContainersAndImages - Autocomplete container names and pod names.
func AutocompleteContainersAndImages(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	containers, _ := getContainers(cmd, toComplete, completeDefault)
	images, _ := getImages(cmd, toComplete)
	return append(containers, images...), cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteVolumes - Autocomplete volumes.
func AutocompleteVolumes(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getVolumes(cmd, toComplete)
}

// AutocompleteSecrets - Autocomplete secrets.
func AutocompleteSecrets(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getSecrets(cmd, toComplete, completeDefault)
}

func AutocompleteSecretCreate(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 1 {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteImages - Autocomplete images.
func AutocompleteImages(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getImages(cmd, toComplete)
}

// AutocompleteCreateRun - Autocomplete only the fist argument as image and then do file completion.
func AutocompleteCreateRun(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if len(args) < 1 {
		// check if the rootfs flag is set
		// if it is set to true provide directory completion
		rootfs, err := cmd.Flags().GetBool("rootfs")
		if err == nil && rootfs {
			return nil, cobra.ShellCompDirectiveFilterDirs
		}
		return getImages(cmd, toComplete)
	}
	// TODO: add path completion for files in the image
	return nil, cobra.ShellCompDirectiveDefault
}

// AutocompleteRegistries - Autocomplete registries.
func AutocompleteRegistries(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getRegistries()
}

// AutocompleteNetworks - Autocomplete networks.
func AutocompleteNetworks(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getNetworks(cmd, toComplete, completeDefault)
}

// AutocompleteDefaultOneArg - Autocomplete path only for the first argument.
func AutocompleteDefaultOneArg(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteCommitCommand - Autocomplete podman commit command args.
func AutocompleteCommitCommand(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if len(args) == 0 {
		return getContainers(cmd, toComplete, completeDefault)
	}
	if len(args) == 1 {
		return getImages(cmd, toComplete)
	}
	// don't complete more than 2 args
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteCpCommand - Autocomplete podman cp command args.
func AutocompleteCpCommand(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if len(args) < 2 {
		containers, _ := getContainers(cmd, toComplete, completeDefault)
		for _, container := range containers {
			// TODO: Add path completion for inside the container if possible
			if strings.HasPrefix(container, toComplete) {
				return containers, cobra.ShellCompDirectiveNoSpace
			}
		}
		// else complete paths
		return nil, cobra.ShellCompDirectiveDefault
	}
	// don't complete more than 2 args
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteExecCommand - Autocomplete podman exec command args.
func AutocompleteExecCommand(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if len(args) == 0 {
		return getContainers(cmd, toComplete, completeDefault, "running")
	}
	return nil, cobra.ShellCompDirectiveDefault
}

// AutocompleteRunlabelCommand - Autocomplete podman container runlabel command args.
func AutocompleteRunlabelCommand(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if len(args) == 0 {
		// FIXME: What labels can we recommend here?
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if len(args) == 1 {
		return getImages(cmd, toComplete)
	}
	return nil, cobra.ShellCompDirectiveDefault
}

// AutocompleteContainerOneArg - Autocomplete containers as fist arg.
func AutocompleteContainerOneArg(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if len(args) == 0 {
		return getContainers(cmd, toComplete, completeDefault)
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteNetworkConnectCmd - Autocomplete podman network connect/disconnect command args.
func AutocompleteNetworkConnectCmd(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return getNetworks(cmd, toComplete, completeDefault)
	}
	if len(args) == 1 {
		return getContainers(cmd, toComplete, completeDefault)
	}
	// don't complete more than 2 args
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteTopCmd - Autocomplete podman top/pod top command args.
func AutocompleteTopCmd(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	latest := cmd.Flags().Lookup("latest")
	// only complete containers/pods as first arg if latest is not set
	if len(args) == 0 && (latest == nil || !latest.Changed) {
		if cmd.Parent().Name() == "pod" {
			// need to complete pods since we are using pod top
			return getPods(cmd, toComplete, completeDefault)
		}
		return getContainers(cmd, toComplete, completeDefault)
	}
	descriptors, err := util.GetContainerPidInformationDescriptors()
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return descriptors, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteInspect - Autocomplete podman inspect.
func AutocompleteInspect(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	containers, _ := getContainers(cmd, toComplete, completeDefault)
	images, _ := getImages(cmd, toComplete)
	pods, _ := getPods(cmd, toComplete, completeDefault)
	networks, _ := getNetworks(cmd, toComplete, completeDefault)
	volumes, _ := getVolumes(cmd, toComplete)
	suggestions := append(containers, images...)
	suggestions = append(suggestions, pods...)
	suggestions = append(suggestions, networks...)
	suggestions = append(suggestions, volumes...)
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteSystemConnections - Autocomplete system connections.
func AutocompleteSystemConnections(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	suggestions := []string{}
	cfg, err := config.ReadCustomConfig()
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for k, v := range cfg.Engine.ServiceDestinations {
		// the URI will be show as description in shells like zsh
		suggestions = append(suggestions, k+"\t"+v.URI)
	}

	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteScp returns a list of connections, images, or both, depending on the amount of arguments
func AutocompleteScp(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	switch len(args) {
	case 0:
		split := strings.SplitN(toComplete, "::", 2)
		if len(split) > 1 {
			imageSuggestions, _ := getImages(cmd, split[1])
			return prefixSlice(split[0]+"::", imageSuggestions), cobra.ShellCompDirectiveNoFileComp
		}
		connectionSuggestions, _ := AutocompleteSystemConnections(cmd, args, toComplete)
		imageSuggestions, _ := getImages(cmd, toComplete)
		totalSuggestions := append(suffixCompSlice("::", connectionSuggestions), imageSuggestions...)
		directive := cobra.ShellCompDirectiveNoFileComp
		// if we have connections do not add a space after the completion
		if len(connectionSuggestions) > 0 {
			directive = cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
		}
		return totalSuggestions, directive
	case 1:
		split := strings.SplitN(args[0], "::", 2)
		if len(split) > 1 {
			if len(split[1]) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			imageSuggestions, _ := getImages(cmd, toComplete)
			return imageSuggestions, cobra.ShellCompDirectiveNoFileComp
		}
		connectionSuggestions, _ := AutocompleteSystemConnections(cmd, args, toComplete)
		return suffixCompSlice("::", connectionSuggestions), cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

/* -------------- Flags ----------------- */

// AutocompleteDetachKeys - Autocomplete detach-keys options.
// -> "ctrl-"
func AutocompleteDetachKeys(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if strings.HasSuffix(toComplete, ",") {
		return []string{toComplete + "ctrl-"}, cobra.ShellCompDirectiveNoSpace
	}
	return []string{"ctrl-"}, cobra.ShellCompDirectiveNoSpace
}

// AutocompleteChangeInstructions - Autocomplete change instructions options for commit and import.
// -> "CMD", "ENTRYPOINT", "ENV", "EXPOSE", "LABEL", "ONBUILD", "STOPSIGNAL", "USER", "VOLUME", "WORKDIR"
func AutocompleteChangeInstructions(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return ChangeCmds, cobra.ShellCompDirectiveNoSpace
}

// AutocompleteImageFormat - Autocomplete image format options.
// -> "oci", "docker"
func AutocompleteImageFormat(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ImageFormat := []string{"oci", "docker"}
	return ImageFormat, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteCreateAttach - Autocomplete create --attach options.
// -> "stdin", "stdout", "stderr"
func AutocompleteCreateAttach(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"stdin", "stdout", "stderr"}, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteNamespace - Autocomplete namespace options.
// -> host,container:[name],ns:[path],private
func AutocompleteNamespace(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	kv := keyValueCompletion{
		"container:": func(s string) ([]string, cobra.ShellCompDirective) { return getContainers(cmd, s, completeDefault) },
		"ns:":        func(s string) ([]string, cobra.ShellCompDirective) { return nil, cobra.ShellCompDirectiveDefault },
		"host":       nil,
		"private":    nil,
	}
	return completeKeyValues(toComplete, kv)
}

// AutocompleteUserNamespace - Autocomplete namespace options.
// -> same as AutocompleteNamespace with "auto", "keep-id" added
func AutocompleteUserNamespace(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	results, directive := AutocompleteNamespace(cmd, args, toComplete)
	results = append(results, "auto", "keep-id")
	return results, directive
}

// AutocompleteCgroupMode - Autocomplete cgroup mode options.
// -> "enabled", "disabled", "no-conmon", "split"
func AutocompleteCgroupMode(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cgroupModes := []string{"enabled", "disabled", "no-conmon", "split"}
	return cgroupModes, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteImageVolume - Autocomplete image volume options.
// -> "bind", "tmpfs", "ignore"
func AutocompleteImageVolume(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	imageVolumes := []string{"bind", "tmpfs", "ignore"}
	return imageVolumes, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteLogDriver - Autocomplete log-driver options.
// -> "journald", "none", "k8s-file", "passthrough"
func AutocompleteLogDriver(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// don't show json-file
	logDrivers := []string{define.JournaldLogging, define.NoLogging, define.KubernetesLogging}
	if !registry.IsRemote() {
		logDrivers = append(logDrivers, define.PassthroughLogging)
	}
	return logDrivers, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteLogOpt - Autocomplete log-opt options.
// -> "path=", "tag="
func AutocompleteLogOpt(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// FIXME: are these the only one? the man page states these but in the current shell completion they are more options
	logOptions := []string{"path=", "tag="}
	if strings.HasPrefix(toComplete, "path=") {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return logOptions, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
}

// AutocompletePullOption - Autocomplete pull options for create and run command.
// -> "always", "missing", "never"
func AutocompletePullOption(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	pullOptions := []string{"always", "missing", "never"}
	return pullOptions, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteRestartOption - Autocomplete restart options for create and run command.
// -> "always", "no", "on-failure", "unless-stopped"
func AutocompleteRestartOption(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	restartOptions := []string{define.RestartPolicyAlways, define.RestartPolicyNo,
		define.RestartPolicyOnFailure, define.RestartPolicyUnlessStopped}
	return restartOptions, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteSecurityOption - Autocomplete security options options.
func AutocompleteSecurityOption(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	kv := keyValueCompletion{
		"apparmor=":         nil,
		"no-new-privileges": nil,
		"seccomp=":          func(s string) ([]string, cobra.ShellCompDirective) { return nil, cobra.ShellCompDirectiveDefault },
		"label=": func(s string) ([]string, cobra.ShellCompDirective) {
			if strings.HasPrefix(s, "d") {
				return []string{"disable"}, cobra.ShellCompDirectiveNoFileComp
			}
			return []string{"user:", "role:", "type:", "level:", "filetype:", "disable"}, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
		},
	}
	return completeKeyValues(toComplete, kv)
}

// AutocompleteStopSignal - Autocomplete stop signal options.
// -> "SIGHUP", "SIGINT", "SIGKILL", "SIGTERM"
func AutocompleteStopSignal(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// FIXME: add more/different signals?
	stopSignals := []string{"SIGHUP", "SIGINT", "SIGKILL", "SIGTERM"}
	return stopSignals, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteSystemdFlag - Autocomplete systemd flag options.
// -> "true", "false", "always"
func AutocompleteSystemdFlag(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	systemd := []string{"true", "false", "always"}
	return systemd, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteUserFlag - Autocomplete user flag based on the names and groups (includes ids after first char) in /etc/passwd and /etc/group files.
// -> user:group
func AutocompleteUserFlag(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if strings.Contains(toComplete, ":") {
		// It would be nice to read the file in the image
		// but at this point we don't know the image.
		file, err := os.Open("/etc/group")
		if err != nil {
			cobra.CompErrorln(err.Error())
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		defer file.Close()

		var groups []string
		scanner := bufio.NewScanner(file)
		user := strings.SplitN(toComplete, ":", 2)[0]
		for scanner.Scan() {
			entries := strings.SplitN(scanner.Text(), ":", 4)
			groups = append(groups, user+":"+entries[0])
			// complete ids after at least one char is given
			if len(user)+1 < len(toComplete) {
				groups = append(groups, user+":"+entries[2])
			}
		}
		if err = scanner.Err(); err != nil {
			cobra.CompErrorln(err.Error())
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return groups, cobra.ShellCompDirectiveNoFileComp
	}

	// It would be nice to read the file in the image
	// but at this point we don't know the image.
	file, err := os.Open("/etc/passwd")
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	defer file.Close()

	var users []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		entries := strings.SplitN(scanner.Text(), ":", 7)
		users = append(users, entries[0]+":")
		// complete ids after at least one char is given
		if len(toComplete) > 0 {
			users = append(users, entries[2]+":")
		}
	}
	if err = scanner.Err(); err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return users, cobra.ShellCompDirectiveNoSpace
}

// AutocompleteMountFlag - Autocomplete mount flag options.
// -> "type=bind,", "type=volume,", "type=tmpfs,"
func AutocompleteMountFlag(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	types := []string{"type=bind,", "type=volume,", "type=tmpfs,"}
	// TODO: Add support for all different options
	return types, cobra.ShellCompDirectiveNoSpace
}

// AutocompleteVolumeFlag - Autocomplete volume flag options.
// -> volumes and paths
func AutocompleteVolumeFlag(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	volumes, _ := getVolumes(cmd, toComplete)
	directive := cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveDefault
	if strings.Contains(toComplete, ":") {
		// add space after second path
		directive = cobra.ShellCompDirectiveDefault
	}
	return volumes, directive
}

// AutocompleteNetworkFlag - Autocomplete network flag options.
func AutocompleteNetworkFlag(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	kv := keyValueCompletion{
		"container:": func(s string) ([]string, cobra.ShellCompDirective) { return getContainers(cmd, s, completeDefault) },
		"ns:": func(_ string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveDefault
		},
		"bridge":  nil,
		"none":    nil,
		"host":    nil,
		"private": nil,
		"slirp4netns:": func(s string) ([]string, cobra.ShellCompDirective) {
			skv := keyValueCompletion{
				"allow_host_loopback=": getBoolCompletion,
				"cidr=":                nil,
				"enable_ipv6=":         getBoolCompletion,
				"mtu=":                 nil,
				"outbound_addr=":       nil,
				"outbound_addr6=":      nil,
				"port_handler=": func(_ string) ([]string, cobra.ShellCompDirective) {
					return []string{"rootlesskit", "slirp4netns"}, cobra.ShellCompDirectiveNoFileComp
				},
			}
			return completeKeyValues(s, skv)
		},
	}

	networks, _ := getNetworks(cmd, toComplete, completeDefault)
	suggestions, dir := completeKeyValues(toComplete, kv)
	// add slirp4netns here it does not work correct if we add it to the kv map
	suggestions = append(suggestions, "slirp4netns")
	return append(networks, suggestions...), dir
}

// AutocompleteFormat - Autocomplete json or a given struct to use for a go template.
// The input can be nil, In this case only json will be autocompleted.
// This function will only work for structs other types are not supported.
// When "{{." is typed the field and method names of the given struct will be completed.
// This also works recursive for nested structs.
func AutocompleteFormat(o interface{}) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// this function provides shell completion for go templates
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// autocomplete json when nothing or json is typed
		if strings.HasPrefix("json", toComplete) {
			return []string{"json"}, cobra.ShellCompDirectiveNoFileComp
		}
		// no input struct we cannot provide completion return nothing
		if o == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		// toComplete could look like this: {{ .Config }} {{ .Field.F
		// 1. split the template variable delimiter
		vars := strings.Split(toComplete, "{{")
		if len(vars) == 1 {
			// no variables return no completion
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		// clean the spaces from the last var
		field := strings.Split(vars[len(vars)-1], " ")
		// split this into it struct field names
		fields := strings.Split(field[len(field)-1], ".")
		f := reflect.ValueOf(o)
		for i := 1; i < len(fields); i++ {
			if f.Kind() == reflect.Ptr {
				f = f.Elem()
			}

			// the only supported type is struct
			if f.Kind() != reflect.Struct {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			// last field get all names to suggest
			if i == len(fields)-1 {
				suggestions := getStructFields(f, fields[i])
				// add the current toComplete value in front so that the shell can complete this correctly
				toCompArr := strings.Split(toComplete, ".")
				toCompArr[len(toCompArr)-1] = ""
				toComplete = strings.Join(toCompArr, ".")
				return prefixSlice(toComplete, suggestions), cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
			}
			// set the next struct field
			f = f.FieldByName(fields[i])
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

// getStructFields reads all struct field names and method names and returns them.
func getStructFields(f reflect.Value, prefix string) []string {
	suggestions := []string{}
	// follow the pointer first
	if f.Kind() == reflect.Ptr {
		f = f.Elem()
	}
	// we only support structs
	if f.Kind() != reflect.Struct {
		return nil
	}
	// loop over all field names
	for j := 0; j < f.NumField(); j++ {
		field := f.Type().Field(j)
		fname := field.Name
		suffix := "}}"
		kind := field.Type.Kind()
		if kind == reflect.Ptr {
			// make sure to read the actual type when it is a pointer
			kind = field.Type.Elem().Kind()
		}
		// when we have a nested struct do not append braces instead append a dot
		if kind == reflect.Struct {
			suffix = "."
		}
		if strings.HasPrefix(fname, prefix) {
			// add field name with suffix
			suggestions = append(suggestions, fname+suffix)
		}
		// if field is anonymous add the child fields as well
		if field.Anonymous {
			suggestions = append(suggestions, getStructFields(f.FieldByIndex([]int{j}), prefix)...)
		}
	}

	for j := 0; j < f.NumMethod(); j++ {
		fname := f.Type().Method(j).Name
		if strings.HasPrefix(fname, prefix) {
			// add method name with closing braces
			suggestions = append(suggestions, fname+"}}")
		}
	}

	return suggestions
}

// AutocompleteEventFilter - Autocomplete event filter flag options.
// -> "container=", "event=", "image=", "pod=", "volume=", "type="
func AutocompleteEventFilter(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	eventTypes := func(_ string) ([]string, cobra.ShellCompDirective) {
		return []string{"attach", "checkpoint", "cleanup", "commit", "create", "exec",
			"export", "import", "init", "kill", "mount", "pause", "prune", "remove",
			"restart", "restore", "start", "stop", "sync", "unmount", "unpause",
			"pull", "push", "save", "tag", "untag", "refresh", "renumber",
		}, cobra.ShellCompDirectiveNoFileComp
	}
	kv := keyValueCompletion{
		"container=": func(s string) ([]string, cobra.ShellCompDirective) { return getContainers(cmd, s, completeDefault) },
		"image=":     func(s string) ([]string, cobra.ShellCompDirective) { return getImages(cmd, s) },
		"pod=":       func(s string) ([]string, cobra.ShellCompDirective) { return getPods(cmd, s, completeDefault) },
		"volume=":    func(s string) ([]string, cobra.ShellCompDirective) { return getVolumes(cmd, s) },
		"event=":     eventTypes,
		"type=":      eventTypes,
	}
	return completeKeyValues(toComplete, kv)
}

// AutocompleteSystemdRestartOptions - Autocomplete systemd restart options.
// -> "no", "on-success", "on-failure", "on-abnormal", "on-watchdog", "on-abort", "always"
func AutocompleteSystemdRestartOptions(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return systemdDefine.RestartPolicies, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteTrustType - Autocomplete trust type options.
// -> "signedBy", "accept", "reject"
func AutocompleteTrustType(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	types := []string{"signedBy", "accept", "reject"}
	return types, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteImageSort - Autocomplete images sort options.
// -> "created", "id", "repository", "size", "tag"
func AutocompleteImageSort(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	sortBy := []string{"created", "id", "repository", "size", "tag"}
	return sortBy, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteInspectType - Autocomplete inspect type options.
// -> "container", "image", "all"
func AutocompleteInspectType(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	types := []string{"container", "image", "all"}
	return types, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteManifestFormat - Autocomplete manifest format options.
// -> "oci", "v2s2"
func AutocompleteManifestFormat(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	types := []string{"oci", "v2s2"}
	return types, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteNetworkDriver - Autocomplete network driver option.
// -> "bridge", "macvlan"
func AutocompleteNetworkDriver(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	drivers := []string{types.BridgeNetworkDriver, types.MacVLANNetworkDriver, types.IPVLANNetworkDriver}
	return drivers, cobra.ShellCompDirectiveNoFileComp
}

// AutocompletePodShareNamespace - Autocomplete pod create --share flag option.
// -> "ipc", "net", "pid", "user", "uts", "cgroup", "none"
func AutocompletePodShareNamespace(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	namespaces := []string{"ipc", "net", "pid", "user", "uts", "cgroup", "none"}
	split := strings.Split(toComplete, ",")
	split[len(split)-1] = ""
	toComplete = strings.Join(split, ",")
	return prefixSlice(toComplete, namespaces), cobra.ShellCompDirectiveNoFileComp
}

// AutocompletePodPsSort - Autocomplete images sort options.
// -> "created", "id", "name", "status", "number"
func AutocompletePodPsSort(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	sortBy := []string{"created", "id", "name", "status", "number"}
	return sortBy, cobra.ShellCompDirectiveNoFileComp
}

// AutocompletePsSort - Autocomplete images sort options.
// -> "command", "created", "id", "image", "names", "runningfor", "size", "status"
func AutocompletePsSort(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	sortBy := []string{"command", "created", "id", "image", "names", "runningfor", "size", "status"}
	return sortBy, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteImageSaveFormat - Autocomplete image save format options.
// -> "oci-archive", "oci-dir", "docker-dir"
func AutocompleteImageSaveFormat(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	formats := []string{"oci-archive", "oci-dir", "docker-dir"}
	return formats, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteWaitCondition - Autocomplete wait condition options.
// -> "unknown", "configured", "created", "running", "stopped", "paused", "exited", "removing"
func AutocompleteWaitCondition(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	states := []string{"unknown", "configured", "created", "running", "stopped", "paused", "exited", "removing"}
	return states, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteCgroupManager - Autocomplete cgroup manager options.
// -> "cgroupfs", "systemd"
func AutocompleteCgroupManager(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	types := []string{"cgroupfs", "systemd"}
	return types, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteEventBackend - Autocomplete event backend options.
// -> "file", "journald", "none"
func AutocompleteEventBackend(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	types := []string{"file", "journald", "none"}
	return types, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteNetworkBackend - Autocomplete network backend options.
// -> "cni", "netavark"
func AutocompleteNetworkBackend(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	types := []string{"cni", "netavark"}
	return types, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteLogLevel - Autocomplete log level options.
// -> "trace", "debug", "info", "warn", "error", "fatal", "panic"
func AutocompleteLogLevel(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return LogLevels, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteSDNotify - Autocomplete sdnotify options.
// -> "container", "conmon", "ignore"
func AutocompleteSDNotify(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	types := []string{"container", "conmon", "ignore"}
	return types, cobra.ShellCompDirectiveNoFileComp
}

var containerStatuses = []string{"created", "running", "paused", "stopped", "exited", "unknown"}

// AutocompletePsFilters - Autocomplete ps filter options.
func AutocompletePsFilters(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	kv := keyValueCompletion{
		"id=":   func(s string) ([]string, cobra.ShellCompDirective) { return getContainers(cmd, s, completeIDs) },
		"name=": func(s string) ([]string, cobra.ShellCompDirective) { return getContainers(cmd, s, completeNames) },
		"status=": func(_ string) ([]string, cobra.ShellCompDirective) {
			return containerStatuses, cobra.ShellCompDirectiveNoFileComp
		},
		"ancestor=": func(s string) ([]string, cobra.ShellCompDirective) { return getImages(cmd, s) },
		"before=":   func(s string) ([]string, cobra.ShellCompDirective) { return getContainers(cmd, s, completeDefault) },
		"since=":    func(s string) ([]string, cobra.ShellCompDirective) { return getContainers(cmd, s, completeDefault) },
		"volume=":   func(s string) ([]string, cobra.ShellCompDirective) { return getVolumes(cmd, s) },
		"health=": func(_ string) ([]string, cobra.ShellCompDirective) {
			return []string{define.HealthCheckHealthy,
				define.HealthCheckUnhealthy}, cobra.ShellCompDirectiveNoFileComp
		},
		"network=": func(s string) ([]string, cobra.ShellCompDirective) { return getNetworks(cmd, s, completeDefault) },
		"label=":   nil,
		"exited=":  nil,
		"until=":   nil,
	}
	return completeKeyValues(toComplete, kv)
}

// AutocompletePodPsFilters - Autocomplete pod ps filter options.
func AutocompletePodPsFilters(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	kv := keyValueCompletion{
		"id=":   func(s string) ([]string, cobra.ShellCompDirective) { return getPods(cmd, s, completeIDs) },
		"name=": func(s string) ([]string, cobra.ShellCompDirective) { return getPods(cmd, s, completeNames) },
		"status=": func(_ string) ([]string, cobra.ShellCompDirective) {
			return []string{"stopped", "running",
				"paused", "exited", "dead", "created", "degraded"}, cobra.ShellCompDirectiveNoFileComp
		},
		"ctr-ids=":    func(s string) ([]string, cobra.ShellCompDirective) { return getContainers(cmd, s, completeIDs) },
		"ctr-names=":  func(s string) ([]string, cobra.ShellCompDirective) { return getContainers(cmd, s, completeNames) },
		"ctr-number=": nil,
		"ctr-status=": func(_ string) ([]string, cobra.ShellCompDirective) {
			return containerStatuses, cobra.ShellCompDirectiveNoFileComp
		},
		"network=": func(s string) ([]string, cobra.ShellCompDirective) { return getNetworks(cmd, s, completeDefault) },
		"label=":   nil,
	}
	return completeKeyValues(toComplete, kv)
}

// AutocompleteImageFilters - Autocomplete image ls --filter options.
func AutocompleteImageFilters(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	getImg := func(s string) ([]string, cobra.ShellCompDirective) { return getImages(cmd, s) }
	kv := keyValueCompletion{
		"before=":    getImg,
		"since=":     getImg,
		"label=":     nil,
		"reference=": nil,
		"dangling=":  getBoolCompletion,
		"readonly=":  getBoolCompletion,
	}
	return completeKeyValues(toComplete, kv)
}

// AutocompletePruneFilters - Autocomplete container/image prune --filter options.
func AutocompletePruneFilters(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	kv := keyValueCompletion{
		"until=": nil,
		"label=": nil,
	}
	return completeKeyValues(toComplete, kv)
}

// AutocompleteNetworkFilters - Autocomplete network ls --filter options.
func AutocompleteNetworkFilters(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	kv := keyValueCompletion{
		"name=":  func(s string) ([]string, cobra.ShellCompDirective) { return getNetworks(cmd, s, completeNames) },
		"id=":    func(s string) ([]string, cobra.ShellCompDirective) { return getNetworks(cmd, s, completeIDs) },
		"label=": nil,
		"driver=": func(_ string) ([]string, cobra.ShellCompDirective) {
			return []string{types.BridgeNetworkDriver, types.MacVLANNetworkDriver, types.IPVLANNetworkDriver}, cobra.ShellCompDirectiveNoFileComp
		},
		"until=": nil,
	}
	return completeKeyValues(toComplete, kv)
}

// AutocompleteVolumeFilters - Autocomplete volume ls --filter options.
func AutocompleteVolumeFilters(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	local := func(_ string) ([]string, cobra.ShellCompDirective) {
		return []string{"local"}, cobra.ShellCompDirectiveNoFileComp
	}
	kv := keyValueCompletion{
		"name=":     func(s string) ([]string, cobra.ShellCompDirective) { return getVolumes(cmd, s) },
		"driver=":   local,
		"scope=":    local,
		"label=":    nil,
		"opt=":      nil,
		"dangling=": getBoolCompletion,
	}
	return completeKeyValues(toComplete, kv)
}

// AutocompleteSecretFilters - Autocomplete secret ls --filter options.
func AutocompleteSecretFilters(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	kv := keyValueCompletion{
		"name=": func(s string) ([]string, cobra.ShellCompDirective) { return getSecrets(cmd, s, completeNames) },
		"id=":   func(s string) ([]string, cobra.ShellCompDirective) { return getSecrets(cmd, s, completeIDs) },
	}
	return completeKeyValues(toComplete, kv)
}

// AutocompleteCheckpointCompressType - Autocomplete checkpoint compress type options.
// -> "gzip", "none", "zstd"
func AutocompleteCheckpointCompressType(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	types := []string{"gzip", "none", "zstd"}
	return types, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteCompressionFormat - Autocomplete compression-format type options.
func AutocompleteCompressionFormat(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	types := []string{"gzip", "zstd", "zstd:chunked"}
	return types, cobra.ShellCompDirectiveNoFileComp
}
