package main

import (
	"os"
	"os/exec"
	"strconv"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	buildFlags = []cli.Flag{
		cli.StringSliceFlag{
			Name:  "build-arg",
			Usage: "`argument=value` to supply to the builder",
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
	}
	buildDescription = "podman build launches the Buildah command to build an OCI Image. Buildah must be installed for this command to work."
	buildCommand     = cli.Command{
		Name:        "build",
		Aliases:     []string{"build"},
		Usage:       "Build an image using instructions in a Dockerfile",
		Description: buildDescription,
		Flags:       buildFlags,
		Action:      buildCmd,
		ArgsUsage:   "CONTEXT-DIRECTORY | URL",
	}
)

func buildCmd(c *cli.Context) error {

	budCmdArgs := []string{"bud"}

	for _, buildArg := range c.StringSlice("build-arg") {
		budCmdArgs = append(budCmdArgs, "--build-arg", buildArg)
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
