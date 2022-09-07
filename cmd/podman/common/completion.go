package common

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	libimageDefine "github.com/containers/common/libimage/define"
	"github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/ssh"
	"github.com/containers/image/v5/pkg/sysregistriesv2"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/signal"
	systemdDefine "github.com/containers/podman/v4/pkg/systemd/define"
	"github.com/containers/podman/v4/pkg/util"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/spf13/cobra"
)

var (
	// ChangeCmds is the list of valid Change commands to passed to the Commit call
	ChangeCmds = []string{"CMD", "ENTRYPOINT", "ENV", "EXPOSE", "LABEL", "ONBUILD", "STOPSIGNAL", "USER", "VOLUME", "WORKDIR"}
	// LogLevels supported by podman
	LogLevels = []string{"trace", "debug", "info", "warn", "warning", "error", "fatal", "panic"}
	// ValidSaveFormats is the list of support podman save formats
	ValidSaveFormats = []string{define.OCIManifestDir, define.OCIArchive, define.V2s2ManifestDir, define.V2s2Archive}
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
	if !registry.IsRemote() {
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
	// is required to set up the rootless namespace
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

func fdIsNotDir(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		cobra.CompErrorln(err.Error())
		return true
	}
	return !stat.IsDir()
}

func getPathCompletion(root string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if toComplete == "" {
		toComplete = "/"
	}
	// Important: securejoin is required to make sure we never leave the root mount point
	userpath, err := securejoin.SecureJoin(root, toComplete)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveDefault
	}
	var base string
	f, err := os.Open(userpath)
	// when error or file is not dir get the parent path to stat
	if err != nil || fdIsNotDir(f) {
		// Do not use path.Dir() since this cleans the paths which
		// then no longer matches the user input.
		userpath, base = path.Split(userpath)
		toComplete, _ = path.Split(toComplete)
		f, err = os.Open(userpath)
		if err != nil {
			return nil, cobra.ShellCompDirectiveDefault
		}
	}

	if fdIsNotDir(f) {
		// nothing to complete since it is no dir
		return nil, cobra.ShellCompDirectiveDefault
	}

	entries, err := f.ReadDir(-1)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveDefault
	}
	if len(entries) == 0 {
		// path is empty dir, just add the trailing slash and no space
		if !strings.HasSuffix(toComplete, "/") {
			toComplete += "/"
		}
		return []string{toComplete}, cobra.ShellCompDirectiveDefault | cobra.ShellCompDirectiveNoSpace
	}
	completions := make([]string, 0, len(entries))
	count := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), base) {
			suf := ""
			// When the entry is an directory we add the "/" as suffix and do not want to add space
			// to match normal shell completion behavior.
			// Just inc counter again to fake more than one entry in this case and thus get no space.
			if e.IsDir() {
				suf = "/"
				count++
			}
			completions = append(completions, simplePathJoinUnix(toComplete, e.Name()+suf))
			count++
		}
	}
	directive := cobra.ShellCompDirectiveDefault
	if count > 1 {
		// when we have more than one match we do not want to add a space after the completion
		directive |= cobra.ShellCompDirectiveNoSpace
	}
	return completions, directive
}

// simplePathJoinUnix joins to path components by adding a slash only if p1 doesn't end with one.
// We cannot use path.Join() for the completions logic because this one always calls Clean() on
// the path which changes it from the input.
func simplePathJoinUnix(p1, p2 string) string {
	if p1[len(p1)-1] == '/' {
		return p1 + p2
	}
	return p1 + "/" + p2
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
			slice[i] += suf
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

// AutoCompletePodsPause - Autocomplete only paused pod names
// When a pod has a few containers paused, that ends up in degraded state
// So autocomplete degraded pod names as well
func AutoCompletePodsPause(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getPods(cmd, toComplete, completeDefault, "paused", "degraded")
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

func AutocompleteForGenerate(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return AutocompleteForKube(cmd, args, toComplete)
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

// AutocompleteImageSearchFilters - Autocomplate `search --filter`.
func AutocompleteImageSearchFilters(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return libimageDefine.SearchFilters, cobra.ShellCompDirectiveNoFileComp
}

// AutocompletePodExitPolicy - Autocomplete pod exit policy.
func AutocompletePodExitPolicy(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return config.PodExitPolicies, cobra.ShellCompDirectiveNoFileComp
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
	// Mount the image and provide path completion
	engine, err := setupImageEngine(cmd)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveDefault
	}

	resp, err := engine.Mount(registry.Context(), []string{args[0]}, entities.ImageMountOptions{})
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveDefault
	}
	defer func() {
		_, err := engine.Unmount(registry.Context(), []string{args[0]}, entities.ImageUnmountOptions{})
		if err != nil {
			cobra.CompErrorln(err.Error())
		}
	}()
	if len(resp) != 1 {
		return nil, cobra.ShellCompDirectiveDefault
	}

	// So this uses ShellCompDirectiveDefault to also still provide normal shell
	// completion in case no path matches. This is useful if someone tries to get
	// completion for paths that are not available in the image, e.g. /proc/...
	return getPathCompletion(resp[0].Path, toComplete)
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
		if i := strings.IndexByte(toComplete, ':'); i > -1 {
			// Looks like the user already set the container.
			// Lets mount it and provide path completion for files in the container.
			engine, err := setupContainerEngine(cmd)
			if err != nil {
				cobra.CompErrorln(err.Error())
				return nil, cobra.ShellCompDirectiveDefault
			}

			resp, err := engine.ContainerMount(registry.Context(), []string{toComplete[:i]}, entities.ContainerMountOptions{})
			if err != nil {
				cobra.CompErrorln(err.Error())
				return nil, cobra.ShellCompDirectiveDefault
			}
			defer func() {
				_, err := engine.ContainerUnmount(registry.Context(), []string{toComplete[:i]}, entities.ContainerUnmountOptions{})
				if err != nil {
					cobra.CompErrorln(err.Error())
				}
			}()
			if len(resp) != 1 {
				return nil, cobra.ShellCompDirectiveDefault
			}
			comps, directive := getPathCompletion(resp[0].Path, toComplete[i+1:])
			return prefixSlice(toComplete[:i+1], comps), directive
		}
		// Suggest containers when they match the input otherwise normal shell completion is used
		containers, _ := getContainers(cmd, toComplete, completeDefault)
		for _, container := range containers {
			if strings.HasPrefix(container, toComplete) {
				return suffixCompSlice(":", containers), cobra.ShellCompDirectiveNoSpace
			}
		}
		// else complete paths on the host
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
		// This is unfortunate because the argument order is label followed by image.
		// If it would be the other way around we could inspect the first arg and get
		// all labels from it to suggest them.
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

	suggestions := make([]string, 0, len(containers)+len(images)+len(pods)+len(networks)+len(volumes))
	suggestions = append(suggestions, containers...)
	suggestions = append(suggestions, images...)
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
	results = append(results, "auto", "keep-id", "nomap")
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
	logOptions := []string{"path=", "tag=", "max-size="}
	if strings.HasPrefix(toComplete, "path=") {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return logOptions, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
}

// AutocompletePullOption - Autocomplete pull options for create and run command.
// -> "always", "missing", "never"
func AutocompletePullOption(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	pullOptions := []string{"always", "missing", "never", "newer"}
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
// Autocompletes signals both lower or uppercase depending on the user input.
func AutocompleteStopSignal(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// convertCase will convert a string to lowercase only if the user input is lowercase
	convertCase := func(s string) string { return s }
	if len(toComplete) > 0 && unicode.IsLower(rune(toComplete[0])) {
		convertCase = strings.ToLower
	}

	prefix := ""
	// if input starts with "SI" we have to add SIG in front
	// since the signal map does not have this prefix but the option
	// allows signals with and without SIG prefix
	if strings.HasPrefix(toComplete, convertCase("SI")) {
		prefix = "SIG"
	}

	stopSignals := make([]string, 0, len(signal.SignalMap))
	for sig := range signal.SignalMap {
		stopSignals = append(stopSignals, convertCase(prefix+sig))
	}
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

type formatSuggestion struct {
	fieldname string
	suffix    string
}

func convertFormatSuggestions(suggestions []formatSuggestion) []string {
	completions := make([]string, 0, len(suggestions))
	for _, f := range suggestions {
		completions = append(completions, f.fieldname+f.suffix)
	}
	return completions
}

// AutocompleteFormat - Autocomplete json or a given struct to use for a go template.
// The input can be nil, In this case only json will be autocompleted.
// This function will only work for pointer to structs other types are not supported.
// When "{{." is typed the field and method names of the given struct will be completed.
// This also works recursive for nested structs.
func AutocompleteFormat(o interface{}) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// this function provides shell completion for go templates
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// autocomplete json when nothing or json is typed
		// gocritic complains about the argument order but this is correct in this case
		//nolint:gocritic
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
		if f.Kind() != reflect.Ptr {
			// We panic here to make sure that all callers pass the value by reference.
			// If someone passes a by value then all podman commands will panic since
			// this function is run at init time.
			panic("AutocompleteFormat: passed value must be a pointer to a struct")
		}
		for i := 1; i < len(fields); i++ {
			// last field get all names to suggest
			if i == len(fields)-1 {
				suggestions := getStructFields(f, fields[i])
				// add the current toComplete value in front so that the shell can complete this correctly
				toCompArr := strings.Split(toComplete, ".")
				toCompArr[len(toCompArr)-1] = ""
				toComplete = strings.Join(toCompArr, ".")
				return prefixSlice(toComplete, convertFormatSuggestions(suggestions)), cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
			}

			// first follow pointer and create element when it is nil
			f = actualReflectValue(f)
			switch f.Kind() {
			case reflect.Struct:
				for j := 0; j < f.NumField(); j++ {
					field := f.Type().Field(j)
					// ok this is a bit weird but when we have an embedded nil struct
					// calling FieldByName on a name which is present on this struct will panic
					// Therefore we have to init them (non nil ptr), https://github.com/containers/podman/issues/14223
					if field.Anonymous && f.Field(j).Type().Kind() == reflect.Ptr {
						f.Field(j).Set(reflect.New(f.Field(j).Type().Elem()))
					}
				}
				// set the next struct field
				f = f.FieldByName(fields[i])
			case reflect.Map:
				rtype := f.Type().Elem()
				if rtype.Kind() == reflect.Ptr {
					rtype = rtype.Elem()
				}
				f = reflect.New(rtype)
			case reflect.Func:
				if f.Type().NumOut() != 1 {
					// unsupported type return nothing
					return nil, cobra.ShellCompDirectiveNoFileComp
				}
				f = reflect.New(f.Type().Out(0))
			default:
				// unsupported type return nothing
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

// actualReflectValue takes the value,
// if it is pointer it will dereference it and when it is nil,
// it will create a new value from it
func actualReflectValue(f reflect.Value) reflect.Value {
	// follow the pointer first
	if f.Kind() == reflect.Ptr {
		// if the pointer is nil we create a new value from the elements type
		// this allows us to follow nil pointers and get the actual type
		if f.IsNil() {
			f = reflect.New(f.Type().Elem())
		}
		f = f.Elem()
	}
	return f
}

// getStructFields reads all struct field names and method names and returns them.
func getStructFields(f reflect.Value, prefix string) []formatSuggestion {
	var suggestions []formatSuggestion
	if f.IsValid() {
		suggestions = append(suggestions, getMethodNames(f, prefix)...)
	}

	f = actualReflectValue(f)
	// we only support structs
	if f.Kind() != reflect.Struct {
		return suggestions
	}

	var anonymous []formatSuggestion
	// loop over all field names
	for j := 0; j < f.NumField(); j++ {
		field := f.Type().Field(j)
		// check if struct field is not exported, templates only use exported fields
		// PkgPath is always empty for exported fields
		if field.PkgPath != "" {
			continue
		}
		fname := field.Name
		suffix := "}}"
		kind := field.Type.Kind()
		if kind == reflect.Ptr {
			// make sure to read the actual type when it is a pointer
			kind = field.Type.Elem().Kind()
		}
		// when we have a nested struct do not append braces instead append a dot
		if kind == reflect.Struct || kind == reflect.Map {
			suffix = "."
		}
		// if field is anonymous add the child fields as well
		if field.Anonymous {
			anonymous = append(anonymous, getStructFields(f.Field(j), prefix)...)
		}
		if strings.HasPrefix(fname, prefix) {
			// add field name with suffix
			suggestions = append(suggestions, formatSuggestion{fieldname: fname, suffix: suffix})
		}
	}
outer:
	for _, ano := range anonymous {
		// we should only add anonymous child fields if they are not already present.
		for _, sug := range suggestions {
			if ano.fieldname == sug.fieldname {
				continue outer
			}
		}
		suggestions = append(suggestions, ano)
	}
	return suggestions
}

func getMethodNames(f reflect.Value, prefix string) []formatSuggestion {
	suggestions := make([]formatSuggestion, 0, f.NumMethod())
	for j := 0; j < f.NumMethod(); j++ {
		method := f.Type().Method(j)
		// in a template we can only run functions with one return value
		if method.Func.Type().NumOut() != 1 {
			continue
		}
		// when we have a nested struct do not append braces instead append a dot
		kind := method.Func.Type().Out(0).Kind()
		suffix := "}}"
		if kind == reflect.Struct || kind == reflect.Map {
			suffix = "."
		}
		// From a template users POV it is not important when the use a struct field or method.
		// They only notice the difference when the function requires arguments.
		// So lets be nice and let the user know that this method requires arguments via the help text.
		// Note since this is actually a method on a type the first argument is always fix so we should skip it.
		num := method.Func.Type().NumIn() - 1
		if num > 0 {
			// everything after tab will the completion scripts show as help when enabled
			// overwrite the suffix because it expects the args
			suffix = "\tThis is a function and requires " + strconv.Itoa(num) + " argument"
			if num > 1 {
				// add plural s
				suffix += "s"
			}
		}
		fname := method.Name
		if strings.HasPrefix(fname, prefix) {
			// add method name with closing braces
			suggestions = append(suggestions, formatSuggestion{fieldname: fname, suffix: suffix})
		}
	}
	return suggestions
}

// AutocompleteEventFilter - Autocomplete event filter flag options.
// -> "container=", "event=", "image=", "pod=", "volume=", "type="
func AutocompleteEventFilter(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	event := func(_ string) ([]string, cobra.ShellCompDirective) {
		return []string{events.Attach.String(), events.AutoUpdate.String(), events.Checkpoint.String(), events.Cleanup.String(),
			events.Commit.String(), events.Create.String(), events.Exec.String(), events.ExecDied.String(),
			events.Exited.String(), events.Export.String(), events.Import.String(), events.Init.String(), events.Kill.String(),
			events.LoadFromArchive.String(), events.Mount.String(), events.NetworkConnect.String(),
			events.NetworkDisconnect.String(), events.Pause.String(), events.Prune.String(), events.Pull.String(),
			events.Push.String(), events.Refresh.String(), events.Remove.String(), events.Rename.String(),
			events.Renumber.String(), events.Restart.String(), events.Restore.String(), events.Save.String(),
			events.Start.String(), events.Stop.String(), events.Sync.String(), events.Tag.String(), events.Unmount.String(),
			events.Unpause.String(), events.Untag.String(),
		}, cobra.ShellCompDirectiveNoFileComp
	}
	eventTypes := func(_ string) ([]string, cobra.ShellCompDirective) {
		return []string{events.Container.String(), events.Image.String(), events.Network.String(),
			events.Pod.String(), events.System.String(), events.Volume.String(),
		}, cobra.ShellCompDirectiveNoFileComp
	}
	kv := keyValueCompletion{
		"container=": func(s string) ([]string, cobra.ShellCompDirective) { return getContainers(cmd, s, completeDefault) },
		"image=":     func(s string) ([]string, cobra.ShellCompDirective) { return getImages(cmd, s) },
		"pod=":       func(s string) ([]string, cobra.ShellCompDirective) { return getPods(cmd, s, completeDefault) },
		"volume=":    func(s string) ([]string, cobra.ShellCompDirective) { return getVolumes(cmd, s) },
		"event=":     event,
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
func AutocompleteInspectType(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	types := []string{AllType, ContainerType, ImageType, NetworkType, PodType, VolumeType}
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

// AutocompleteNetworkIPAMDriver - Autocomplete network ipam driver option.
// -> "bridge", "macvlan"
func AutocompleteNetworkIPAMDriver(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	drivers := []string{types.HostLocalIPAMDriver, types.DHCPIPAMDriver, types.NoneIPAMDriver}
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
func AutocompleteImageSaveFormat(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return ValidSaveFormats, cobra.ShellCompDirectiveNoFileComp
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
	types := []string{config.CgroupfsCgroupsManager, config.SystemdCgroupsManager}
	return types, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteEventBackend - Autocomplete event backend options.
// -> "file", "journald", "none"
func AutocompleteEventBackend(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	types := []string{events.LogFile.String(), events.Journald.String(), events.Null.String()}
	return types, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteNetworkBackend - Autocomplete network backend options.
// -> "cni", "netavark"
func AutocompleteNetworkBackend(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	types := []string{string(types.CNI), string(types.Netavark)}
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
	types := []string{define.SdNotifyModeContainer, define.SdNotifyModeContainer, define.SdNotifyModeIgnore}
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

// AutocompleteClone - Autocomplete container and image names
func AutocompleteClone(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	switch len(args) {
	case 0:
		if cmd.Parent().Name() == "pod" { // needs to be " pod " to exclude 'podman'
			return getPods(cmd, toComplete, completeDefault)
		}
		return getContainers(cmd, toComplete, completeDefault)
	case 2:
		if cmd.Parent().Name() == "pod" {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return getImages(cmd, toComplete)
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteSSH - Autocomplete ssh modes
func AutocompleteSSH(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !validCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return []string{string(ssh.GolangMode), string(ssh.NativeMode)}, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteHealthOnFailure - action to take once the container turns unhealthy.
func AutocompleteHealthOnFailure(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return define.SupportedHealthCheckOnFailureActions, cobra.ShellCompDirectiveNoFileComp
}
