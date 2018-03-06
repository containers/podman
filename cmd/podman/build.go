package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	buildFlags = []cli.Flag{
		// The following flags are emulated from:
		// src/github.com/projectatomic/buildah/cmd/bud.go
		cli.StringFlag{
			Name:  "authfile",
			Usage: "path of the authentication file. Default is ${XDG_RUNTIME_DIR}/containers/auth.json",
		},
		cli.StringSliceFlag{
			Name:  "build-arg",
			Usage: "`argument=value` to supply to the builder",
		},
		cli.StringFlag{
			Name:  "cert-dir",
			Value: "",
			Usage: "use certificates at the specified path to access the registry",
		},
		cli.StringFlag{
			Name:  "creds",
			Value: "",
			Usage: "use `[username[:password]]` for accessing the registry",
		},
		cli.StringSliceFlag{
			Name:  "file, f",
			Usage: "`pathname or URL` of a Dockerfile",
		},
		cli.StringFlag{
			Name:  "format",
			Usage: "`format` of the built image's manifest and metadata",
		},
		cli.BoolFlag{
			Name:  "pull-always",
			Usage: "pull the image, even if a version is present",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "refrain from announcing build instructions and image read/write progress",
		},
		cli.StringFlag{
			Name:  "runtime",
			Usage: "`path` to an alternate runtime",
		},
		cli.StringSliceFlag{
			Name:  "runtime-flag",
			Usage: "add global flags for the container runtime",
		},
		cli.StringFlag{
			Name:  "signature-policy",
			Usage: "`pathname` of signature policy file (not usually used)",
		},
		cli.StringSliceFlag{
			Name:  "tag, t",
			Usage: "`tag` to apply to the built image",
		},
		cli.BoolFlag{
			Name:  "tls-verify",
			Usage: "require HTTPS and verify certificates when accessing the registry",
		},
		// The following flags are emulated from:
		// src/github.com/projectatomic/buildah/cmd/common.go fromAndBudFlags
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
			Name:  "cpu-shares",
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
			Usage: "security Options (default [])",
		},
		cli.StringFlag{
			Name:  "shm-size",
			Usage: "size of `/dev/shm`. The format is `<number><unit>`.",
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
	}
	buildDescription = "podman build launches the Buildah command to build an OCI Image. Buildah must be installed for this command to work."
	buildCommand     = cli.Command{
		Name:        "build",
		Usage:       "Build an image using instructions in a Dockerfile",
		Description: buildDescription,
		Flags:       buildFlags,
		Action:      buildCmd,
		ArgsUsage:   "CONTEXT-DIRECTORY | URL",
	}
)

func buildCmd(c *cli.Context) error {

	budCmdArgs := []string{}

	// Handle Global Options
	logLevel := c.GlobalString("log-level")
	if logLevel == "debug" {
		budCmdArgs = append(budCmdArgs, "--debug")
	}
	if c.GlobalIsSet("root") {
		budCmdArgs = append(budCmdArgs, "--root", c.GlobalString("root"))
	}
	if c.GlobalIsSet("runroot") {
		budCmdArgs = append(budCmdArgs, "--runroot", c.GlobalString("runroot"))
	}
	if c.GlobalIsSet("storage-driver") {
		budCmdArgs = append(budCmdArgs, "--storage-driver", c.GlobalString("storage-driver"))
	}
	for _, storageOpt := range c.GlobalStringSlice("storage-opt") {
		budCmdArgs = append(budCmdArgs, "--storage-opt", storageOpt)
	}

	budCmdArgs = append(budCmdArgs, "bud")

	// Buildah bud specific options
	if c.IsSet("authfile") {
		budCmdArgs = append(budCmdArgs, "--authfile", c.String("authfile"))
	}
	for _, buildArg := range c.StringSlice("build-arg") {
		budCmdArgs = append(budCmdArgs, "--build-arg", buildArg)
	}
	if c.IsSet("cert-dir") {
		budCmdArgs = append(budCmdArgs, "--cert-dir", c.String("cert-dir"))
	}
	if c.IsSet("creds") {
		budCmdArgs = append(budCmdArgs, "--creds", c.String("creds"))
	}
	for _, fileName := range c.StringSlice("file") {
		budCmdArgs = append(budCmdArgs, "--file", fileName)
	}
	if c.IsSet("format") {
		budCmdArgs = append(budCmdArgs, "--format", c.String("format"))
	}
	if c.IsSet("pull-always") {
		budCmdArgs = append(budCmdArgs, "--pull-always")
	}
	if c.IsSet("quiet") {
		quietParam := "--quiet=" + strconv.FormatBool(c.Bool("quiet"))
		budCmdArgs = append(budCmdArgs, quietParam)
	}
	if c.IsSet("runtime") {
		budCmdArgs = append(budCmdArgs, "--runtime", c.String("runtime"))
	}
	for _, runtimeArg := range c.StringSlice("runtime-flag") {
		budCmdArgs = append(budCmdArgs, "--runtime-flag", runtimeArg)
	}
	if c.IsSet("signature-policy") {
		budCmdArgs = append(budCmdArgs, "--signature-policy", c.String("signature-policy"))
	}
	for _, tagArg := range c.StringSlice("tag") {
		budCmdArgs = append(budCmdArgs, "--tag", tagArg)
	}
	if c.IsSet("tls-verify") {
		tlsParam := "--tls-verify=" + strconv.FormatBool(c.Bool("tls-verify"))
		budCmdArgs = append(budCmdArgs, tlsParam)
	}

	// Buildah bud and from options from cmd/buildah/common.go
	for _, addHostArg := range c.StringSlice("add-host") {
		budCmdArgs = append(budCmdArgs, "--add-host", addHostArg)
	}
	if c.IsSet("cgroup-parent") {
		budCmdArgs = append(budCmdArgs, "--cgroup-parent", c.String("cgroup-parent"))
	}
	if c.IsSet("cpu-period") {
		budCmdArgs = append(budCmdArgs, "--cpu-period", fmt.Sprintf("%v", c.Int64("cpu-period")))
	}
	if c.IsSet("cpu-quota") {
		budCmdArgs = append(budCmdArgs, "--cpu-quota", fmt.Sprintf("%v", c.Uint64("cpu-quota")))
	}
	if c.IsSet("cpu-shares") {
		budCmdArgs = append(budCmdArgs, "--cpu-shares", fmt.Sprintf("%v", c.Uint64("cpu-shares")))
	}
	if c.IsSet("cpuset-cpus") {
		budCmdArgs = append(budCmdArgs, "--cpuset-cpus", c.String("cpuset-cpus"))
	}
	if c.IsSet("cpuset-mems") {
		budCmdArgs = append(budCmdArgs, "--cpuset-mems", c.String("cpuset-mems"))
	}
	if c.IsSet("memory") {
		budCmdArgs = append(budCmdArgs, "--memory", c.String("memory"))
	}
	if c.IsSet("memory-swap") {
		budCmdArgs = append(budCmdArgs, "--memory-swap", c.String("memory-swap"))
	}
	for _, securityOptArg := range c.StringSlice("security-opt") {
		budCmdArgs = append(budCmdArgs, "--security-opt", securityOptArg)
	}
	if c.IsSet("shm-size") {
		budCmdArgs = append(budCmdArgs, "--shm-size", c.String("shm-size"))
	}
	for _, ulimitArg := range c.StringSlice("ulimit") {
		budCmdArgs = append(budCmdArgs, "--ulimit", ulimitArg)
	}
	for _, volumeArg := range c.StringSlice("volume") {
		budCmdArgs = append(budCmdArgs, "--volume", volumeArg)
	}

	if len(c.Args()) > 0 {
		budCmdArgs = append(budCmdArgs, c.Args()...)
	}

	buildah := "buildah"

	if _, err := exec.LookPath(buildah); err != nil {
		return errors.Wrapf(err, "buildah not found in PATH")
	}
	if _, err := exec.Command(buildah).Output(); err != nil {
		return errors.Wrapf(err, "buildah is not operational on this server")
	}

	cmd := exec.Command(buildah, budCmdArgs...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "error running the buildah build-using-dockerfile (bud) command")
	}

	return nil
}
