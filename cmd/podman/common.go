package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/containers/buildah"
	buildahcli "github.com/containers/buildah/pkg/cli"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/sysinfo"
	"github.com/fatih/camelcase"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	json = jsoniter.ConfigCompatibleWithStandardLibrary
)

const (
	idTruncLength      = 12
	sizeWithUnitFormat = "(format: `<number>[<unit>]`, where unit = b (bytes), k (kilobytes), m (megabytes), or g (gigabytes))"
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
func checkAllAndLatest(c *cobra.Command, args []string, ignoreArgLen bool) error {
	argLen := len(args)
	if c.Flags().Lookup("all") == nil || c.Flags().Lookup("latest") == nil {
		return errors.New("unable to lookup values for 'latest' or 'all'")
	}
	all, _ := c.Flags().GetBool("all")
	latest, _ := c.Flags().GetBool("latest")
	if all && latest {
		return errors.Errorf("--all and --latest cannot be used together")
	}
	if ignoreArgLen {
		return nil
	}
	if (all || latest) && argLen > 0 {
		return errors.Errorf("no arguments are needed with --all or --latest")
	}
	if argLen < 1 && !all && !latest {
		return errors.Errorf("you must provide at least one name or id")
	}
	return nil
}

// noSubArgs checks that there are no further positional parameters
func noSubArgs(c *cobra.Command, args []string) error {
	if len(args) > 0 {
		return errors.Errorf("`%s` takes no arguments", c.CommandPath())
	}
	return nil
}

func commandRunE() func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return errors.Errorf("unrecognized command `%s %s`\nTry '%s --help' for more information.", cmd.CommandPath(), args[0], cmd.CommandPath())
		} else {
			return errors.Errorf("missing command '%s COMMAND'\nTry '%s --help' for more information.", cmd.CommandPath(), cmd.CommandPath())
		}
	}
}

// getContext returns a non-nil, empty context
func getContext() context.Context {
	if Ctx != nil {
		return Ctx
	}
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
		"authfile", buildahcli.GetDefaultAuthFile(),
		"Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override",
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
		"cgroupns", "",
		"cgroup namespace to use",
	)
	createFlags.String(
		"cgroups", "enabled",
		"control container cgroup configuration",
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
		"detach-keys", define.DefaultDetachKeys,
		"Override the key sequence for detaching a container. Format is a single character `[a-Z]` or a comma separated sequence of `ctrl-<value>`, where `<value>` is one of: `a-z`, `@`, `^`, `[`, `\\`, `]`, `^` or `_`",
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
	createFlags.StringArrayP(
		"env", "e", []string{},
		"Set environment variables in container",
	)
	createFlags.Bool(
		"env-host", false, "Use all current host environment variables in container",
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
	createFlags.String(
		"health-cmd", "",
		"set a healthcheck command for the container ('none' disables the existing healthcheck)",
	)
	createFlags.String(
		"health-interval", cliconfig.DefaultHealthCheckInterval,
		"set an interval for the healthchecks (a value of disable results in no automatic timer setup)",
	)
	createFlags.Uint(
		"health-retries", cliconfig.DefaultHealthCheckRetries,
		"the number of retries allowed before a healthcheck is considered to be unhealthy",
	)
	createFlags.String(
		"health-start-period", cliconfig.DefaultHealthCheckStartPeriod,
		"the initialization time needed for a container to bootstrap",
	)
	createFlags.String(
		"health-timeout", cliconfig.DefaultHealthCheckTimeout,
		"the maximum time allowed to complete the healthcheck before an interval is considered failed",
	)
	createFlags.StringP(
		"hostname", "h", "",
		"Set container hostname",
	)
	createFlags.Bool(
		"http-proxy", true,
		"Set proxy environment variables in the container based on the host proxy vars",
	)
	createFlags.String(
		"image-volume", cliconfig.DefaultImageVolume,
		"Tells podman how to handle the builtin image volumes. The options are: 'bind', 'tmpfs', or 'ignore'",
	)
	createFlags.Bool(
		"init", false,
		"Run an init binary inside the container that forwards signals and reaps processes",
	)
	createFlags.String(
		"init-path", "",
		// Do not use  the Value field for setting the default value to determine user input (i.e., non-empty string)
		fmt.Sprintf("Path to the container-init binary (default: %q)", define.DefaultInitPath),
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
		"Kernel memory limit "+sizeWithUnitFormat,
	)
	createFlags.StringArrayP(
		"label", "l", []string{},
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
		"Container MAC address (e.g. 92:d0:c6:0a:29:33)",
	)
	createFlags.StringP(
		"memory", "m", "",
		"Memory limit "+sizeWithUnitFormat,
	)
	createFlags.String(
		"memory-reservation", "",
		"Memory soft limit "+sizeWithUnitFormat,
	)
	createFlags.String(
		"memory-swap", "",
		"Swap limit equal to memory plus swap: '-1' to enable unlimited swap",
	)
	createFlags.Int64(
		"memory-swappiness", -1,
		"Tune container memory swappiness (0 to 100, or -1 for system default)",
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
		"no-hosts", false,
		"Do not create /etc/hosts within the container, instead use the version from the image",
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
		"override-arch", "",
		"use `ARCH` instead of the architecture of the machine for choosing images",
	)
	markFlagHidden(createFlags, "override-arch")
	createFlags.String(
		"override-os", "",
		"use `OS` instead of the running OS for choosing images",
	)
	markFlagHidden(createFlags, "override-os")
	createFlags.String(
		"pid", "",
		"PID namespace to use",
	)
	createFlags.Int64(
		"pids-limit", sysinfo.GetDefaultPidsLimit(),
		"Tune container pids limit (set 0 for unlimited)",
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
	createFlags.String(
		"pull", "missing",
		`Pull image before creating ("always"|"missing"|"never") (default "missing")`,
	)
	createFlags.BoolP(
		"quiet", "q", false,
		"Suppress output information when pulling images",
	)
	createFlags.Bool(
		"read-only", false,
		"Make containers root filesystem read-only",
	)
	createFlags.Bool(
		"read-only-tmpfs", true,
		"When running containers in read-only mode mount a read-write tmpfs on /run, /tmp and /var/tmp",
	)
	createFlags.String(
		"restart", "",
		"Restart policy to apply when a container exits",
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
		"shm-size", cliconfig.DefaultShmSize,
		"Size of /dev/shm "+sizeWithUnitFormat,
	)
	createFlags.String(
		"stop-signal", "",
		"Signal to stop a container. Default is SIGTERM",
	)
	createFlags.Uint(
		"stop-timeout", define.CtrRemoveTimeout,
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
	createFlags.String(
		"systemd", "true",
		`Run container in systemd mode ("true"|"false"|"always" (default "true")`,
	)
	createFlags.StringArray(
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
		"userns", os.Getenv("PODMAN_USERNS"),
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

// scrubServer removes 'http://' or 'https://' from the front of the
// server/registry string if either is there.  This will be mostly used
// for user input from 'podman login' and 'podman logout'.
func scrubServer(server string) string {
	server = strings.TrimPrefix(server, "https://")
	return strings.TrimPrefix(server, "http://")
}

// HelpTemplate returns the help template for podman commands
// This uses the short and long options.
// command should not use this.
func HelpTemplate() string {
	return `{{.Short}}

Description:
  {{.Long}}

{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`
}

// UsageTemplate returns the usage template for podman commands
// This blocks the desplaying of the global options. The main podman
// command should not use this.
func UsageTemplate() string {
	return `Usage:{{if (and .Runnable (not .HasAvailableSubCommands))}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
  {{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}
{{end}}
`
}
