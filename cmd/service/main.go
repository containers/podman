package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/pkg/serviceapi"
	"github.com/spf13/cobra"
)

func initConfig() {
	//	we can do more stuff in here.
}

func main() {
	cobra.OnInitialize(initConfig)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(50)*time.Millisecond)
	defer cancel()

	config := cliconfig.PodmanCommand{
		Command:     &cobra.Command{},
		InputArgs:   []string{},
		GlobalFlags: cliconfig.MainFlags{},
		Remote:      false,
	}
	// Create a single runtime for http
	runtime, err := libpodruntime.GetRuntimeDisableFDs(context.Background(), &config)
	if err != nil {
		fmt.Printf("error creating libpod runtime: %s", err.Error())
		os.Exit(1)
	}
	defer runtime.DeferredShutdown(false)

	server, err := serviceapi.NewServer(runtime)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	err = server.Serve()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	defer func() {
		if e := server.Shutdown(ctx); e != nil {
			fmt.Println(err.Error())
		}
	}()
}
