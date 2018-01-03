package main

import (
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/containers/storage"
	"github.com/fatih/camelcase"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libkpod"
	"github.com/projectatomic/libpod/libpod"
	"github.com/urfave/cli"
)

const crioConfigPath = "/etc/crio/crio.conf"

func getRuntime(c *cli.Context) (*libpod.Runtime, error) {

	config, err := getConfig(c)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get config")
	}

	options := storage.DefaultStoreOptions
	options.GraphRoot = config.Root
	options.RunRoot = config.RunRoot
	options.GraphDriverName = config.Storage
	options.GraphDriverOptions = config.StorageOptions

	return libpod.NewRuntime(libpod.WithStorageConfig(options), libpod.WithConmonPath(config.Conmon), libpod.WithOCIRuntime(config.Runtime), libpod.WithCNIConfigDir(config.NetworkDir))
}

func getConfig(c *cli.Context) (*libkpod.Config, error) {
	config := libkpod.DefaultConfig()
	var configFile string
	if c.GlobalIsSet("config") {
		configFile = c.GlobalString("config")
	} else if _, err := os.Stat(crioConfigPath); err == nil {
		configFile = crioConfigPath
	}
	// load and merge the configfile from the commandline or use
	// the default crio config file
	if configFile != "" {
		err := config.UpdateFromFile(configFile)
		if err != nil {
			return config, err
		}
	}
	if c.GlobalIsSet("root") {
		config.Root = c.GlobalString("root")
	}
	if c.GlobalIsSet("runroot") {
		config.RunRoot = c.GlobalString("runroot")
	}
	if c.GlobalIsSet("conmon") {
		config.Conmon = c.GlobalString("conmon")
	}
	if c.GlobalIsSet("storage-driver") {
		config.Storage = c.GlobalString("storage-driver")
	}
	if c.GlobalIsSet("storage-opt") {
		opts := c.GlobalStringSlice("storage-opt")
		if len(opts) > 0 {
			config.StorageOptions = opts
		}
	}
	if c.GlobalIsSet("runtime") {
		config.Runtime = c.GlobalString("runtime")
	}
	if c.GlobalIsSet("cni-config-dir") {
		config.NetworkDir = c.GlobalString("cni-config-dir")
	}
	return config, nil
}

func splitCamelCase(src string) string {
	entries := camelcase.Split(src)
	return strings.Join(entries, " ")
}

// validateFlags searches for StringFlags or StringSlice flags that never had
// a value set.  This commonly occurs when the CLI mistakenly takes the next
// option and uses it as a value.
func validateFlags(c *cli.Context, flags []cli.Flag) error {
	for _, flag := range flags {
		switch reflect.TypeOf(flag).String() {
		case "cli.StringSliceFlag":
			{
				f := flag.(cli.StringSliceFlag)
				name := strings.Split(f.Name, ",")
				val := c.StringSlice(name[0])
				for _, v := range val {
					if ok, _ := regexp.MatchString("^-.+", v); ok {
						return errors.Errorf("option --%s requires a value", name[0])
					}
				}
			}
		case "cli.StringFlag":
			{
				f := flag.(cli.StringFlag)
				name := strings.Split(f.Name, ",")
				val := c.String(name[0])
				if ok, _ := regexp.MatchString("^-.+", val); ok {
					return errors.Errorf("option --%s requires a value", name[0])
				}
			}
		}
	}
	return nil
}

// Common flags shared between commands
var createFlags = []cli.Flag{
	cli.StringSliceFlag{
		Name:  "add-host",
		Usage: "Add a custom host-to-IP mapping (host:ip) (default [])",
	},
	cli.StringSliceFlag{
		Name:  "attach, a",
		Usage: "Attach to STDIN, STDOUT or STDERR (default [])",
	},
	cli.StringFlag{
		Name:  "blkio-weight",
		Usage: "Block IO weight (relative weight) accepts a weight value between 10 and 1000.",
	},
	cli.StringSliceFlag{
		Name:  "blkio-weight-device",
		Usage: "Block IO weight (relative device weight, format: `DEVICE_NAME:WEIGHT`)",
	},
	cli.StringSliceFlag{
		Name:  "cap-add",
		Usage: "Add capabilities to the container",
	},
	cli.StringSliceFlag{
		Name:  "cap-drop",
		Usage: "Drop capabilities from the container",
	},
	cli.StringFlag{
		Name:  "cgroup-parent",
		Usage: "Optional parent cgroup for the container",
	},
	cli.StringFlag{
		Name:  "cidfile",
		Usage: "Write the container ID to the file",
	},
	cli.Uint64Flag{
		Name:  "cpu-period",
		Usage: "Limit the CPU CFS (Completely Fair Scheduler) period",
	},
	cli.Int64Flag{
		Name:  "cpu-quota",
		Usage: "Limit the CPU CFS (Completely Fair Scheduler) quota",
	},
	cli.Uint64Flag{
		Name:  "cpu-rt-period",
		Usage: "Limit the CPU real-time period in microseconds",
	},
	cli.Int64Flag{
		Name:  "cpu-rt-runtime",
		Usage: "Limit the CPU real-time runtime in microseconds",
	},
	cli.Uint64Flag{
		Name:  "cpu-shares",
		Usage: "CPU shares (relative weight)",
	},
	cli.StringFlag{
		Name:  "cpus",
		Usage: "Number of CPUs. The default is 0.000 which means no limit",
	},
	cli.StringFlag{
		Name:  "cpuset-cpus",
		Usage: "CPUs in which to allow execution (0-3, 0,1)",
	},
	cli.StringFlag{
		Name:  "cpuset-mems",
		Usage: "Memory nodes (MEMs) in which to allow execution (0-3, 0,1). Only effective on NUMA systems.",
	},
	cli.BoolFlag{
		Name:  "detach, d",
		Usage: "Run container in background and print container ID",
	},
	cli.StringFlag{
		Name:  "detach-keys",
		Usage: "Override the key sequence for detaching a container. Format is a single character `[a-Z]` or `ctrl-<value>` where `<value>` is one of: `a-z`, `@`, `^`, `[`, `,` or `_`",
	},
	cli.StringSliceFlag{
		Name:  "device",
		Usage: "Add a host device to the container (default [])",
	},
	cli.StringSliceFlag{
		Name:  "device-read-bps",
		Usage: "Limit read rate (bytes per second) from a device (e.g. --device-read-bps=/dev/sda:1mb)",
	},
	cli.StringSliceFlag{
		Name:  "device-read-iops",
		Usage: "Limit read rate (IO per second) from a device (e.g. --device-read-iops=/dev/sda:1000)",
	},
	cli.StringSliceFlag{
		Name:  "device-write-bps",
		Usage: "Limit write rate (bytes per second) to a device (e.g. --device-write-bps=/dev/sda:1mb)",
	},
	cli.StringSliceFlag{
		Name:  "device-write-iops",
		Usage: "Limit write rate (IO per second) to a device (e.g. --device-write-iops=/dev/sda:1000)",
	},
	cli.StringSliceFlag{
		Name:  "dns",
		Usage: "Set custom DNS servers",
	},
	cli.StringSliceFlag{
		Name:  "dns-opt",
		Usage: "Set custom DNS options",
	},
	cli.StringSliceFlag{
		Name:  "dns-search",
		Usage: "Set custom DNS search domains",
	},
	cli.StringFlag{
		Name:  "entrypoint",
		Usage: "Overwrite the default ENTRYPOINT of the image",
	},
	cli.StringSliceFlag{
		Name:  "env, e",
		Usage: "Set environment variables in container",
	},
	cli.StringSliceFlag{
		Name:  "env-file",
		Usage: "Read in a file of environment variables",
	},
	cli.StringSliceFlag{
		Name:  "expose",
		Usage: "Expose a port or a range of ports (default [])",
	},
	cli.StringSliceFlag{
		Name:  "group-add",
		Usage: "Add additional groups to join (default [])",
	},
	cli.StringFlag{
		Name:  "hostname",
		Usage: "Set container hostname",
	},
	cli.BoolFlag{
		Name:  "interactive, i",
		Usage: "Keep STDIN open even if not attached",
	},
	cli.StringFlag{
		Name:  "ip",
		Usage: "Container IPv4 address (e.g. 172.23.0.9)",
	},
	cli.StringFlag{
		Name:  "ip6",
		Usage: "Container IPv6 address (e.g. 2001:db8::1b99)",
	},
	cli.StringFlag{
		Name:  "ipc",
		Usage: "IPC Namespace to use",
	},
	cli.StringFlag{
		Name:  "kernel-memory",
		Usage: "Kernel memory limit (format: `<number>[<unit>]`, where unit = b, k, m or g)",
	},
	cli.StringSliceFlag{
		Name:  "label",
		Usage: "Set metadata on container (default [])",
	},
	cli.StringSliceFlag{
		Name:  "label-file",
		Usage: "Read in a line delimited file of labels (default [])",
	},
	cli.StringSliceFlag{
		Name:  "link-local-ip",
		Usage: "Container IPv4/IPv6 link-local addresses (default [])",
	},
	cli.StringFlag{
		Name:  "log-driver",
		Usage: "Logging driver for the container",
	},
	cli.StringSliceFlag{
		Name:  "log-opt",
		Usage: "Logging driver options (default [])",
	},
	cli.StringFlag{
		Name:  "mac-address",
		Usage: "Container MAC address (e.g. 92:d0:c6:0a:29:33)",
	},
	cli.StringFlag{
		Name:  "memory, m",
		Usage: "Memory limit (format: <number>[<unit>], where unit = b, k, m or g)",
	},
	cli.StringFlag{
		Name:  "memory-reservation",
		Usage: "Memory soft limit (format: <number>[<unit>], where unit = b, k, m or g)",
	},
	cli.StringFlag{
		Name:  "memory-swap",
		Usage: "Swap limit equal to memory plus swap: '-1' to enable unlimited swap",
	},
	cli.Int64Flag{
		Name:  "memory-swappiness",
		Usage: "Tune container memory swappiness (0 to 100) (default -1)",
		Value: -1,
	},
	cli.StringFlag{
		Name:  "name",
		Usage: "Assign a name to the container",
	},
	cli.StringFlag{
		Name:  "net",
		Usage: "Setup the network namespace",
	},
	cli.StringFlag{
		Name:  "network",
		Usage: "Connect a container to a network",
		Value: "bridge",
	},
	cli.StringSliceFlag{
		Name:  "network-alias",
		Usage: "Add network-scoped alias for the container (default [])",
	},
	cli.BoolFlag{
		Name:  "oom-kill-disable",
		Usage: "Disable OOM Killer",
	},
	cli.StringFlag{
		Name:  "oom-score-adj",
		Usage: "Tune the host's OOM preferences (-1000 to 1000)",
	},
	cli.StringFlag{
		Name:  "pid",
		Usage: "PID Namespace to use",
	},
	cli.Int64Flag{
		Name:  "pids-limit",
		Usage: "Tune container pids limit (set -1 for unlimited)",
	},
	cli.StringFlag{
		Name:  "pod",
		Usage: "Run container in an existing pod",
	},
	cli.BoolFlag{
		Name:  "privileged",
		Usage: "Give extended privileges to container",
	},
	cli.StringSliceFlag{
		Name:  "publish, p",
		Usage: "Publish a container's port, or a range of ports, to the host (default [])",
	},
	cli.BoolFlag{
		Name:  "publish-all, P",
		Usage: "Publish all exposed ports to random ports on the host interface",
	},
	cli.BoolFlag{
		Name:  "quiet, q",
		Usage: "Suppress output information when pulling images",
	},
	cli.BoolFlag{
		Name:  "read-only",
		Usage: "Make containers root filesystem read-only",
	},
	cli.BoolFlag{
		Name:  "rm",
		Usage: "Remove container (and pod if created) after exit",
	},
	cli.StringSliceFlag{
		Name:  "security-opt",
		Usage: "Security Options (default [])",
	},
	cli.StringFlag{
		Name:  "shm-size",
		Usage: "Size of `/dev/shm`. The format is `<number><unit>`. default is 64 MB",
	},
	cli.BoolFlag{
		Name:  "sig-proxy",
		Usage: "Proxy received signals to the process (default true)",
	},
	cli.StringFlag{
		Name:  "stop-signal",
		Usage: "Signal to stop a container. Default is SIGTERM",
	},
	cli.IntFlag{
		Name:  "stop-timeout",
		Usage: "Timeout (in seconds) to stop a container. Default is 10",
		Value: libpod.CtrRemoveTimeout,
	},
	cli.StringSliceFlag{
		Name:  "storage-opt",
		Usage: "Storage driver options per container (default [])",
	},
	cli.StringSliceFlag{
		Name:  "sysctl",
		Usage: "Sysctl options (default [])",
	},
	cli.StringSliceFlag{
		Name:  "tmpfs",
		Usage: "Mount a temporary filesystem (`tmpfs`) into a container (default [])",
	},
	cli.BoolFlag{
		Name:  "tty, t",
		Usage: "Allocate a pseudo-TTY for container",
	},
	cli.StringSliceFlag{
		Name:  "ulimit",
		Usage: "Ulimit options (default [])",
	},
	cli.StringFlag{
		Name:  "user, u",
		Usage: "Username or UID (format: <name|uid>[:<group|gid>])",
	},
	cli.StringFlag{
		Name:  "userns",
		Usage: "User namespace to use",
	},
	cli.StringFlag{
		Name:  "uts",
		Usage: "UTS namespace to use",
	},
	cli.StringSliceFlag{
		Name:  "volume, v",
		Usage: "Bind mount a volume into the container (default [])",
	},
	cli.StringSliceFlag{
		Name:  "volumes-from",
		Usage: "Mount volumes from the specified container(s) (default [])",
	},
	cli.StringFlag{
		Name:  "workdir, w",
		Usage: "Working `directory inside the container",
		Value: "/",
	},
}
