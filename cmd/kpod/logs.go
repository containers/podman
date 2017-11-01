package main

import (
	"fmt"
	"time"

	"github.com/kubernetes-incubator/cri-o/libkpod"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	logsFlags = []cli.Flag{
		cli.BoolFlag{
			Name:   "details",
			Usage:  "Show extra details provided to the logs",
			Hidden: true,
		},
		cli.BoolFlag{
			Name:  "follow, f",
			Usage: "Follow log output.  The default is false",
		},
		cli.StringFlag{
			Name:  "since",
			Usage: "Show logs since TIMESTAMP",
		},
		cli.Uint64Flag{
			Name:  "tail",
			Usage: "Output the specified number of LINES at the end of the logs.  Defaults to 0, which prints all lines",
		},
	}
	logsDescription = "The kpod logs command batch-retrieves whatever logs are present for a container at the time of execution.  This does not guarantee execution" +
		"order when combined with kpod run (i.e. your run may not have generated any logs at the time you execute kpod logs"
	logsCommand = cli.Command{
		Name:        "logs",
		Usage:       "Fetch the logs of a container",
		Description: logsDescription,
		Flags:       logsFlags,
		Action:      logsCmd,
		ArgsUsage:   "CONTAINER",
	}
)

func logsCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) != 1 {
		return errors.Errorf("'kpod logs' requires exactly one container name/ID")
	}
	if err := validateFlags(c, logsFlags); err != nil {
		return err
	}
	container := c.Args().First()
	var opts libkpod.LogOptions
	opts.Details = c.Bool("details")
	opts.Follow = c.Bool("follow")
	opts.SinceTime = time.Time{}
	if c.IsSet("since") {
		// parse time, error out if something is wrong
		since, err := time.Parse("2006-01-02T15:04:05.999999999-07:00", c.String("since"))
		if err != nil {
			return errors.Wrapf(err, "could not parse time: %q", c.String("since"))
		}
		opts.SinceTime = since
	}
	opts.Tail = c.Uint64("tail")

	config, err := getConfig(c)
	if err != nil {
		return errors.Wrapf(err, "could not get config")
	}
	server, err := libkpod.New(config)
	if err != nil {
		return errors.Wrapf(err, "could not create container server")
	}
	defer server.Shutdown()
	err = server.Update()
	if err != nil {
		return errors.Wrapf(err, "could not update list of containers")
	}
	logs := make(chan string)
	go func() {
		err = server.GetLogs(container, logs, opts)
	}()
	printLogs(logs)
	return err
}

func printLogs(logs chan string) {
	for line := range logs {
		fmt.Println(line)
	}
}
