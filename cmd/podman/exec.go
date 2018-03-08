package main

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	"github.com/projectatomic/libpod/pkg/util"
	"github.com/urfave/cli"
)

var (
	execFlags = []cli.Flag{
		cli.StringSliceFlag{
			Name:  "env, e",
			Usage: "Set environment variables",
		},
		cli.BoolFlag{
			Name:  "privileged",
			Usage: "Give the process extended Linux capabilities inside the container.  The default is false",
		},
		cli.BoolFlag{
			Name:  "tty, t",
			Usage: "Allocate a pseudo-TTY. The default is false",
		},
		cli.StringFlag{
			Name:  "user, u",
			Usage: "Sets the username or UID used and optionally the groupname or GID for the specified command",
		},
		LatestFlag,
	}
	execDescription = `
	podman exec

	Run a command in a running container
`

	execCommand = cli.Command{
		Name:           "exec",
		Usage:          "Run a process in a running container",
		Description:    execDescription,
		Flags:          execFlags,
		Action:         execCmd,
		ArgsUsage:      "CONTAINER-NAME",
		SkipArgReorder: true,
	}
)

func execCmd(c *cli.Context) error {
	var envs []string
	args := c.Args()
	var ctr *libpod.Container
	var err error
	argStart := 1
	if len(args) < 1 && !c.Bool("latest") {
		return errors.Errorf("you must provide one container name or id")
	}
	if len(args) < 2 && !c.Bool("latest") {
		return errors.Errorf("you must provide a command to exec")
	}
	if c.Bool("latest") {
		argStart = 0
	}
	cmd := args[argStart:]
	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	if c.Bool("latest") {
		ctr, err = runtime.GetLatestContainer()
	} else {
		ctr, err = runtime.LookupContainer(args[0])
	}
	if err != nil {
		return errors.Wrapf(err, "unable to exec into %s", args[0])
	}
	// Create a list of keys provided by the user
	var userEnvKeys []string
	for _, env := range c.StringSlice("env") {
		splitEnv := strings.Split(env, "=")
		userEnvKeys = append(userEnvKeys, splitEnv[0])
	}

	envs = append(envs, c.StringSlice("env")...)

	// if the default key isnt in the user-provided list, add the default
	// key and value to the environment variables.  this is needed to set
	// PATH for example.
	for k, v := range defaultEnvVariables {
		if !util.StringInSlice(k, userEnvKeys) {
			envs = append(envs, fmt.Sprintf("%s=%s", k, v))
		}
	}

	return ctr.Exec(c.Bool("tty"), c.Bool("privileged"), envs, cmd, c.String("user"))
}
