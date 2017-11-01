package main

import (
	"fmt"

	"github.com/kubernetes-incubator/cri-o/libkpod"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
)

var (
	rmFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "force, f",
			Usage: "Force removal of a running container.  The default is false",
		},
	}
	rmDescription = "Remove one or more containers"
	rmCommand     = cli.Command{
		Name: "rm",
		Usage: fmt.Sprintf(`kpod rm will remove one or more containers from the host.  The container name or ID can be used.
							This does not remove images.  Running containers will not be removed without the -f option.`),
		Description: rmDescription,
		Flags:       rmFlags,
		Action:      rmCmd,
		ArgsUsage:   "",
	}
)

// saveCmd saves the image to either docker-archive or oci
func rmCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return errors.Errorf("specify one or more containers to remove")
	}
	if err := validateFlags(c, rmFlags); err != nil {
		return err
	}

	config, err := getConfig(c)
	if err != nil {
		return errors.Wrapf(err, "could not get config")
	}
	server, err := libkpod.New(config)
	if err != nil {
		return errors.Wrapf(err, "could not get container server")
	}
	defer server.Shutdown()
	err = server.Update()
	if err != nil {
		return errors.Wrapf(err, "could not update list of containers")
	}
	force := c.Bool("force")

	for _, container := range c.Args() {
		id, err2 := server.Remove(context.Background(), container, force)
		if err2 != nil {
			if err == nil {
				err = err2
			} else {
				err = errors.Wrapf(err, "%v.  Stop the container before attempting removal or use -f\n", err2)
			}
		} else {
			fmt.Println(id)
		}
	}
	return err
}
