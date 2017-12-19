package main

import (
	"os"
	"os/exec"

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
			Usage: "Require HTTPS and verify certificates when accessing the registry",
		},
	}
	buildDescription = "This command launches the Buildah command to build an OCI Image. Buildah must be installed for this command to work."
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

	if c.IsSet("build-arg") {
		budCmdArgs = append(budCmdArgs, "--build-arg")
		budCmdArgs = append(budCmdArgs, c.StringSlice("build-arg")...)
	}
	if c.IsSet("file") || c.IsSet("f") {
		for _, fileName := range c.StringSlice("file") {
			budCmdArgs = append(budCmdArgs, "--file", fileName)
		}
	}
	if c.IsSet("format") {
		budCmdArgs = append(budCmdArgs, "--format", c.String("format"))
	}
	if c.IsSet("pull-always") {
		budCmdArgs = append(budCmdArgs, "--pull-always")
	}
	if c.IsSet("quiet") || c.IsSet("q") {
		boolValue:="False"
		if c.Bool("quiet") {
			boolValue="True"
		}
		budCmdArgs = append(budCmdArgs, "--quiet", boolValue)
	}
	if c.IsSet("runtime") {
		budCmdArgs = append(budCmdArgs, "--runtime", c.String("runtime"))
	}
	if c.IsSet("runtime-flag") {
		budCmdArgs = append(budCmdArgs, "--runtime-flag")
                budCmdArgs = append(budCmdArgs, c.StringSlice("runtime-flag")...)
	}
	if c.IsSet("signature-policy") {
		budCmdArgs = append(budCmdArgs, "--signature-policy", c.String("signature-policy"))
	}
	for _, tagName := range c.StringSlice("tag") {
		budCmdArgs = append(budCmdArgs, "--tag", tagName)
	}
	if c.IsSet("tls-verify") {
		boolValue:="False"
		if c.Bool("tls-verify") {
			boolValue="True"
		}
		budCmdArgs = append(budCmdArgs, "--tls-verify", boolValue)
	}

	if len(c.Args()) > 0 {
		budCmdArgs = append(budCmdArgs, c.Args()...)
	}

	buildah := "buildah"

	_, err := exec.LookPath("buildah")
	if err != nil {
		return errors.Wrapf(err, "buildah not found in PATH")
	}
	_, err = exec.Command(buildah).Output()
	if err != nil {
		return errors.Wrapf(err, "buildah is not operational on this server")
	}

	cmd := exec.Command(buildah, budCmdArgs...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err = cmd.Run(); err != nil {
		return errors.Wrapf(err, "error running the buildah build-using-dockerfile (bud) command")
	}

	return nil
}
