package cli

// the cli package contains urfave/cli related structs that help make up
// the command line for buildah commands. it resides here so other projects
// that vendor in this code can use them too.

import (
	"fmt"
	"os"
	"strings"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/projectatomic/buildah"
	"github.com/projectatomic/buildah/util"
	"github.com/urfave/cli"
)

var (
	usernsFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "userns",
			Usage: "'container', `path` of user namespace to join, or 'host'",
		},
		cli.StringSliceFlag{
			Name:  "userns-uid-map",
			Usage: "`containerID:hostID:length` UID mapping to use in user namespace",
		},
		cli.StringSliceFlag{
			Name:  "userns-gid-map",
			Usage: "`containerID:hostID:length` GID mapping to use in user namespace",
		},
		cli.StringFlag{
			Name:  "userns-uid-map-user",
			Usage: "`name` of entries from /etc/subuid to use to set user namespace UID mapping",
		},
		cli.StringFlag{
			Name:  "userns-gid-map-group",
			Usage: "`name` of entries from /etc/subgid to use to set user namespace GID mapping",
		},
	}

	NamespaceFlags = []cli.Flag{
		cli.StringFlag{
			Name:  string(specs.IPCNamespace),
			Usage: "'container', `path` of IPC namespace to join, or 'host'",
		},
		cli.StringFlag{
			Name:  string(specs.NetworkNamespace) + ", net",
			Usage: "'container', `path` of network namespace to join, or 'host'",
		},
		cli.StringFlag{
			Name:  "cni-config-dir",
			Usage: "`directory` of CNI configuration files",
			Value: util.DefaultCNIConfigDir,
		},
		cli.StringFlag{
			Name:  "cni-plugin-path",
			Usage: "`path` of CNI network plugins",
			Value: util.DefaultCNIPluginPath,
		},
		cli.StringFlag{
			Name:  string(specs.PIDNamespace),
			Usage: "'container', `path` of PID namespace to join, or 'host'",
		},
		cli.StringFlag{
			Name:  string(specs.UTSNamespace),
			Usage: "'container', `path` of UTS namespace to join, or 'host'",
		},
	}

	BudFlags = []cli.Flag{
		cli.StringSliceFlag{
			Name:  "annotation",
			Usage: "Set metadata for an image (default [])",
		},
		cli.StringFlag{
			Name:  "authfile",
			Usage: "path of the authentication file. Default is ${XDG_RUNTIME_DIR}/containers/auth.json",
		},
		cli.StringSliceFlag{
			Name:  "build-arg",
			Usage: "`argument=value` to supply to the builder",
		},
		cli.StringFlag{
			Name:  "cache-from",
			Usage: "Images to utilise as potential cache sources. The build process does not currently support caching so this is a NOOP.",
		},
		cli.StringFlag{
			Name:  "cert-dir",
			Value: "",
			Usage: "use certificates at the specified path to access the registry",
		},
		cli.BoolFlag{
			Name:  "compress",
			Usage: "This is legacy option, which has no effect on the image",
		},
		cli.StringFlag{
			Name:  "creds",
			Value: "",
			Usage: "use `[username[:password]]` for accessing the registry",
		},
		cli.BoolFlag{
			Name:  "disable-content-trust",
			Usage: "This is a Docker specific option and is a NOOP",
		},
		cli.StringSliceFlag{
			Name:  "file, f",
			Usage: "`pathname or URL` of a Dockerfile",
		},
		cli.BoolFlag{
			Name:  "force-rm",
			Usage: "Always remove intermediate containers after a build. The build process does not currently support caching so this is a NOOP.",
		},
		cli.StringFlag{
			Name:  "format",
			Usage: "`format` of the built image's manifest and metadata",
		},
		cli.StringFlag{
			Name:  "iidfile",
			Usage: "`file` to write the image ID to",
		},
		cli.StringSliceFlag{
			Name:  "label",
			Usage: "Set metadata for an image (default [])",
		},
		cli.BoolFlag{
			Name:  "layers",
			Usage: fmt.Sprintf("cache intermediate layers during build (default %t)", UseLayers()),
		},
		cli.BoolFlag{
			Name:  "no-cache",
			Usage: "Do not use existing cached images for the container build. Build from the start with a new set of cached layers.",
		},
		cli.StringFlag{
			Name:  "logfile",
			Usage: "log to `file` instead of stdout/stderr",
		},
		cli.BoolTFlag{
			Name:  "pull",
			Usage: "pull the image if not present",
		},
		cli.BoolFlag{
			Name:  "pull-always",
			Usage: "pull the image, even if a version is present",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "refrain from announcing build instructions and image read/write progress",
		},
		cli.BoolFlag{
			Name:  "rm",
			Usage: "Remove intermediate containers after a successful build. The build process does not currently support caching so this is a NOOP.",
		},
		cli.StringFlag{
			Name:  "runtime",
			Usage: "`path` to an alternate runtime",
			Value: buildah.DefaultRuntime,
		},
		cli.StringSliceFlag{
			Name:  "runtime-flag",
			Usage: "add global flags for the container runtime",
		},
		cli.StringFlag{
			Name:  "signature-policy",
			Usage: "`pathname` of signature policy file (not usually used)",
		},
		cli.BoolFlag{
			Name:  "squash",
			Usage: "Squash newly built layers into a single new layer. The build process does not currently support caching so this is a NOOP.",
		},
		cli.BoolTFlag{
			Name:  "stream",
			Usage: "There is no daemon in use, so this command is a NOOP.",
		},
		cli.StringSliceFlag{
			Name:  "tag, t",
			Usage: "tagged `name` to apply to the built image",
		},
		cli.BoolTFlag{
			Name:  "tls-verify",
			Usage: "require HTTPS and verify certificates when accessing the registry",
		},
	}

	FromAndBudFlags = append(append([]cli.Flag{
		cli.StringSliceFlag{
			Name:  "add-host",
			Usage: "add a custom host-to-IP mapping (host:ip) (default [])",
		},
		cli.StringFlag{
			Name:  "cgroup-parent",
			Usage: "optional parent cgroup for the container",
		},
		cli.Uint64Flag{
			Name:  "cpu-period",
			Usage: "limit the CPU CFS (Completely Fair Scheduler) period",
		},
		cli.Int64Flag{
			Name:  "cpu-quota",
			Usage: "limit the CPU CFS (Completely Fair Scheduler) quota",
		},
		cli.Uint64Flag{
			Name:  "cpu-shares, c",
			Usage: "CPU shares (relative weight)",
		},
		cli.StringFlag{
			Name:  "cpuset-cpus",
			Usage: "CPUs in which to allow execution (0-3, 0,1)",
		},
		cli.StringFlag{
			Name:  "cpuset-mems",
			Usage: "memory nodes (MEMs) in which to allow execution (0-3, 0,1). Only effective on NUMA systems.",
		},
		cli.StringFlag{
			Name:  "memory, m",
			Usage: "memory limit (format: <number>[<unit>], where unit = b, k, m or g)",
		},
		cli.StringFlag{
			Name:  "memory-swap",
			Usage: "swap limit equal to memory plus swap: '-1' to enable unlimited swap",
		},
		cli.StringSliceFlag{
			Name:  "security-opt",
			Usage: "security options (default [])",
		},
		cli.StringFlag{
			Name:  "shm-size",
			Usage: "size of '/dev/shm'. The format is `<number><unit>`.",
			Value: "65536k",
		},
		cli.StringSliceFlag{
			Name:  "ulimit",
			Usage: "ulimit options (default [])",
		},
		cli.StringSliceFlag{
			Name:  "volume, v",
			Usage: "bind mount a volume into the container (default [])",
		},
	}, usernsFlags...), NamespaceFlags...)
)

// UseLayers returns true if BUILDAH_LAYERS is set to "1" or "true"
// otherwise it returns false
func UseLayers() bool {
	layers := os.Getenv("BUILDAH_LAYERS")
	if strings.ToLower(layers) == "true" || layers == "1" {
		return true
	}
	return false
}
