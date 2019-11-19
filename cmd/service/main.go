package main

import (
	"context"
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/pkg/serviceapi"
	"github.com/containers/storage/pkg/reexec"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func initConfig() {
	//	we can do more stuff in here.
}

func main() {
	if reexec.Init() {
		// We were invoked with a different argv[0] indicating that we
		// had a specific job to do as a subprocess, and it's done.
		return
	}

	cobra.OnInitialize(initConfig)
	log.SetLevel(log.DebugLevel)

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
}
