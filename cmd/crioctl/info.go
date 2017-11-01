package main

import (
	"encoding/json"
	"fmt"

	"github.com/kubernetes-incubator/cri-o/client"
	"github.com/urfave/cli"
)

var infoCommand = cli.Command{
	Name:  "info",
	Usage: "get crio daemon info",
	Action: func(context *cli.Context) error {
		c, err := client.New(context.GlobalString("connect"))
		if err != nil {
			return err
		}
		di, err := c.DaemonInfo()
		if err != nil {
			return err
		}

		jsonBytes, err := json.MarshalIndent(di, "", "    ")
		if err != nil {
			return err
		}
		fmt.Println(string(jsonBytes))
		return nil
	},
}
