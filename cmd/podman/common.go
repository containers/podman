package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/containers/buildah"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/storage"
	"github.com/fatih/camelcase"
	"github.com/pkg/errors"
)

var (
	stores = make(map[storage.Store]struct{})
)

const (
	idTruncLength = 12
)

func splitCamelCase(src string) string {
	entries := camelcase.Split(src)
	return strings.Join(entries, " ")
}

func shortID(id string) string {
	if len(id) > idTruncLength {
		return id[:idTruncLength]
	}
	return id
}

// checkAllAndLatest checks that --all and --latest are used correctly
func checkAllAndLatest(c *cliconfig.PodmanCommand) error {
	argLen := len(c.InputArgs)
	if (c.Bool("all") || c.Bool("latest")) && argLen > 0 {
		return errors.Errorf("no arguments are needed with --all or --latest")
	}
	if c.Bool("all") && c.Bool("latest") {
		return errors.Errorf("--all and --latest cannot be used together")
	}
	if argLen < 1 && !c.Bool("all") && !c.Bool("latest") {
		return errors.Errorf("you must provide at least one pod name or id")
	}
	return nil
}

// getAllOrLatestContainers tries to return the correct list of containers
// depending if --all, --latest or <container-id> is used.
// It requires the Context (c) and the Runtime (runtime). As different
// commands are using different container state for the --all option
// the desired state has to be specified in filterState. If no filter
// is desired a -1 can be used to get all containers. For a better
// error message, if the filter fails, a corresponding verb can be
// specified which will then appear in the error message.
func getAllOrLatestContainers(c *cliconfig.PodmanCommand, runtime *libpod.Runtime, filterState libpod.ContainerStatus, verb string) ([]*libpod.Container, error) {
	var containers []*libpod.Container
	var lastError error
	var err error
	if c.Bool("all") {
		if filterState != -1 {
			var filterFuncs []libpod.ContainerFilter
			filterFuncs = append(filterFuncs, func(c *libpod.Container) bool {
				state, _ := c.State()
				return state == filterState
			})
			containers, err = runtime.GetContainers(filterFuncs...)
		} else {
			containers, err = runtime.GetContainers()
		}
		if err != nil {
			return nil, errors.Wrapf(err, "unable to get %s containers", verb)
		}
	} else if c.Bool("latest") {
		lastCtr, err := runtime.GetLatestContainer()
		if err != nil {
			return nil, errors.Wrapf(err, "unable to get latest container")
		}
		containers = append(containers, lastCtr)
	} else {
		args := c.InputArgs
		for _, i := range args {
			container, err := runtime.LookupContainer(i)
			if err != nil {
				if lastError != nil {
					fmt.Fprintln(os.Stderr, lastError)
				}
				lastError = errors.Wrapf(err, "unable to find container %s", i)
			}
			if container != nil {
				// This is here to make sure this does not return [<nil>] but only nil
				containers = append(containers, container)
			}
		}
	}

	return containers, lastError
}

// getContext returns a non-nil, empty context
func getContext() context.Context {
	return context.TODO()
}

func getDefaultNetwork() string {
	if rootless.IsRootless() {
		return "slirp4netns"
	}
	return "bridge"
}

func getCreateFlags(c *cliconfig.PodmanCommand) {

	createFlags := c.Flags()

	createFlags.StringSlice(
		"add-host", []string{},
		"Add a custom host-to-IP mapping (host:ip) (default [])",
	)
	createFlags.StringSlice(
		"annotation", []string{},
		"Add annotations to container (key:value) (default [])",
	)
	createFlags.StringSliceP(
		"attach", "a", []string{},
		"Attach to STDIN, STDOUT or STDERR (default [])",
	)
	createFlags.String(
		"blkio-weight", "",
		"Block IO weight (relative weight) accepts a weight value between 10 and 1000.",
	)
	createFlags.StringSlice(
		"blkio-weight-device", []string{},
		"Block IO weight (relative device weight, format: `DEVICE_NAME:WEIGHT`)",
	)
	createFlags.StringSlice(
		"cap-add", []string{},
		"Add capabilities to the container",
	)
	createFlags.StringSlice(
		"cap-drop", []string{},
		"Drop capabilities from the container",
	)
	createFlags.String(
		"cgroup-parent", "",
		"Optional parent cgroup for the container",
	)
	createFlags.String(
		"cidfile", "",
		"Write the container ID to the file",
	)
	createFlags.String(
		"conmon-pidfile", "",
		"Path to the file that will receive the PID of conmon",
	)
	createFlags.Uint64(
		"cpu-period", 0,
		"Limit the CPU CFS (Completely Fair Scheduler) period",
	)
	createFlags.Int64(
		"cpu-quota", 0,
		"Limit the CPU CFS (Completely Fair Scheduler) quota",
	)
	createFlags.Uint64(
		"cpu-rt-period", 0,
		"Limit the CPU real-time period in microseconds",
	)
	createFlags.Int64(
		"cpu-rt-runtime", 0,
		"Limit the CPU real-time runtime in microseconds",
	)
	createFlags.Uint64(
		"cpu-shares", 0,
		"CPU shares (relative weight)",
	)
	createFlags.Float64(
		"cpus", 0,
		"Number of CPUs. The default is 0.000 which means no limit",
	)
	createFlags.String(
		"cpuset-cpus", "",
		"CPUs in which to allow execution (0-3, 0,1)",
	)
	createFlags.String(
		"cpuset-mems", "",
		"Memory nodes (MEMs) in which to allow execution (0-3, 0,1). Only effective on NUMA systems.",
	)
	createFlags.BoolP(
		"detach", "d", false,
		"Run container in background and print container ID",
	)
	createFlags.String(
		"detach-keys", "",
		"Override the key sequence for detaching a container. Format is a single character `[a-Z]` or `ctrl-<value>` where `<value>` is one of: `a-z`, `@`, `^`, `[`, `,` or `_`",
	)
	createFlags.StringSlice(
		"device", []string{},
		"Add a host device to the container (default [])",
	)
	createFlags.StringSlice(
		"device-read-bps", []string{},
		"Limit read rate (bytes per second) from a device (e.g. --device-read-bps=/dev/sda:1mb)",
	)
	createFlags.StringSlice(
		"device-read-iops", []string{},
		"Limit read rate (IO per second) from a device (e.g. --device-read-iops=/dev/sda:1000)",
	)
	createFlags.StringSlice(
		"device-write-bps", []string{},
		"Limit write rate (bytes per second) to a device (e.g. --device-write-bps=/dev/sda:1mb)",
	)
	createFlags.StringSlice(
		"device-write-iops", []string{},
		"Limit write rate (IO per second) to a device (e.g. --device-write-iops=/dev/sda:1000)",
	)
	createFlags.StringSlice(
		"dns", []string{},
		"Set custom DNS servers",
	)
	createFlags.StringSlice(
		"dns-opt", []string{},
		"Set custom DNS options",
	)
	createFlags.StringSlice(
		"dns-search", []string{},
		"Set custom DNS search domains",
	)
	createFlags.String(
		"entrypoint", "",
		"Overwrite the default ENTRYPOINT of the image",
	)
	createFlags.StringSliceP(
		"env", "e", []string{},
		"Set environment variables in container",
	)
	createFlags.StringSlice(
		"env-file", []string{},
		"Read in a file of environment variables",
	)
	createFlags.StringSlice(
		"expose", []string{},
		"Expose a port or a range of ports (default [])",
	)
	createFlags.StringSlice(
		"gidmap", []string{},
		"GID map to use for the user namespace",
	)
	createFlags.StringSlice(
		"group-add", []string{},
		"Add additional groups to join (default [])",
	)
	createFlags.Bool(
		"help", false, "",
	)

	createFlags.StringP(
		"hostname", "h", "",
		"Set container hostname",
	)
	createFlags.String(
		"image-volume", "bind",
		"Tells podman how to handle the builtin image volumes. The options are: 'bind', 'tmpfs', or 'ignore' (default 'bind')",
	)
	createFlags.Bool(
		"init", false,
		"Run an init binary inside the container that forwards signals and reaps processes",
	)
	createFlags.String(
		"init-path", "",
		// Do not use  the Value field for setting the default value to determine user input (i.e., non-empty string)
		fmt.Sprintf("Path to the container-init binary (default: %q)", libpod.DefaultInitPath),
	)
	createFlags.BoolP(
		"interactive", "i", false,
		"Keep STDIN open even if not attached",
	)
	createFlags.String(
		"ip", "",
		"Specify a static IPv4 address for the container",
	)
	createFlags.String(
		"ipc", "",
		"IPC namespace to use",
	)
	createFlags.String(
		"kernel-memory", "",
		"Kernel memory limit (format: `<number>[<unit>]`, where unit = b, k, m or g)",
	)
	createFlags.StringSlice(
		"label", []string{},
		"Set metadata on container (default [])",
	)
	createFlags.StringSlice(
		"label-file", []string{},
		"Read in a line delimited file of labels (default [])",
	)
	createFlags.String(
		"log-driver", "",
		"Logging driver for the container",
	)
	createFlags.StringSlice(
		"log-opt", []string{},
		"Logging driver options (default [])",
	)
	createFlags.String(
		"mac-address", "",
		"Container MAC address (e.g. 92:d0:c6:0a:29:33), not currently supported",
	)
	createFlags.StringP(
		"memory", "m", "",
		"Memory limit (format: <number>[<unit>], where unit = b, k, m or g)",
	)
	createFlags.String(
		"memory-reservation", "",
		"Memory soft limit (format: <number>[<unit>], where unit = b, k, m or g)",
	)
	createFlags.String(
		"memory-swap", "",
		"Swap limit equal to memory plus swap: '-1' to enable unlimited swap",
	)
	createFlags.Int64(
		"memory-swappiness", -1,
		"Tune container memory swappiness (0 to 100) (default -1)",
	)
	createFlags.String(
		"name", "",
		"Assign a name to the container",
	)
	createFlags.String(
		"net", getDefaultNetwork(),
		"Connect a container to a network",
	)
	createFlags.String(
		"network", getDefaultNetwork(),
		"Connect a container to a network",
	)
	createFlags.Bool(
		"oom-kill-disable", false,
		"Disable OOM Killer",
	)
	createFlags.Int(
		"oom-score-adj", 0,
		"Tune the host's OOM preferences (-1000 to 1000)",
	)
	createFlags.String(
		"pid", "",
		"PID namespace to use",
	)
	createFlags.Int64(
		"pids-limit", 0,
		"Tune container pids limit (set -1 for unlimited)",
	)
	createFlags.String(
		"pod", "",
		"Run container in an existing pod",
	)
	createFlags.Bool(
		"privileged", false,
		"Give extended privileges to container",
	)
	createFlags.StringSliceP(
		"publish", "p", []string{},
		"Publish a container's port, or a range of ports, to the host (default [])",
	)
	createFlags.BoolP(
		"publish-all", "P", false,
		"Publish all exposed ports to random ports on the host interface",
	)
	createFlags.BoolP(
		"quiet", "q", false,
		"Suppress output information when pulling images",
	)
	createFlags.Bool(
		"read-only", false,
		"Make containers root filesystem read-only",
	)
	createFlags.String(
		"restart", "",
		"Restart is not supported.  Please use a systemd unit file for restart",
	)
	createFlags.Bool(
		"rm", false,
		"Remove container (and pod if created) after exit",
	)
	createFlags.Bool(
		"rootfs", false,
		"The first argument is not an image but the rootfs to the exploded container",
	)
	createFlags.StringArray(
		"security-opt", []string{},
		"Security Options (default [])",
	)
	createFlags.String(
		"shm-size", "65536k",
		"Size of `/dev/shm`. The format is `<number><unit>`",
	)
	createFlags.String(
		"stop-signal", "",
		"Signal to stop a container. Default is SIGTERM",
	)
	createFlags.Int(
		"stop-timeout", libpod.CtrRemoveTimeout,
		"Timeout (in seconds) to stop a container. Default is 10",
	)
	createFlags.StringSlice(
		"storage-opt", []string{},
		"Storage driver options per container (default [])",
	)
	createFlags.String(
		"subgidname", "",
		"Name of range listed in /etc/subgid for use in user namespace",
	)
	createFlags.String(
		"subuidname", "",
		"Name of range listed in /etc/subuid for use in user namespace",
	)

	createFlags.StringSlice(
		"sysctl", []string{},
		"Sysctl options (default [])",
	)
	createFlags.Bool(
		"systemd", true,
		"Run container in systemd mode if the command executable is systemd or init",
	)
	createFlags.StringSlice(
		"tmpfs", []string{},
		"Mount a temporary filesystem (`tmpfs`) into a container (default [])",
	)
	createFlags.BoolP(
		"tty", "t", false,
		"Allocate a pseudo-TTY for container",
	)
	createFlags.StringSlice(
		"uidmap", []string{},
		"UID map to use for the user namespace",
	)
	createFlags.StringSlice(
		"ulimit", []string{},
		"Ulimit options (default [])",
	)
	createFlags.StringP(
		"user", "u", "",
		"Username or UID (format: <name|uid>[:<group|gid>])",
	)
	createFlags.String(
		"userns", "",
		"User namespace to use",
	)
	createFlags.String(
		"uts", "",
		"UTS namespace to use",
	)
	createFlags.StringArray(
		"mount", []string{},
		"Attach a filesystem mount to the container (default [])",
	)
	createFlags.StringArrayP(
		"volume", "v", []string{},
		"Bind mount a volume into the container (default [])",
	)
	createFlags.StringSlice(
		"volumes-from", []string{},
		"Mount volumes from the specified container(s) (default [])",
	)
	createFlags.StringP(
		"workdir", "w", "",
		"Working directory inside the container",
	)
}

func getFormat(c *cliconfig.PodmanCommand) (string, error) {
	format := strings.ToLower(c.String("format"))
	if strings.HasPrefix(format, buildah.OCI) {
		return buildah.OCIv1ImageManifest, nil
	}

	if strings.HasPrefix(format, buildah.DOCKER) {
		return buildah.Dockerv2ImageManifest, nil
	}
	return "", errors.Errorf("unrecognized image type %q", format)
}

func getAuthFile(authfile string) string {
	if authfile != "" {
		return authfile
	}
	return os.Getenv("REGISTRY_AUTH_FILE")
}

// scrubServer removes 'http://' or 'https://' from the front of the
// server/registry string if either is there.  This will be mostly used
// for user input from 'podman login' and 'podman logout'.
func scrubServer(server string) string {
	server = strings.TrimPrefix(server, "https://")
	return strings.TrimPrefix(server, "http://")
}
