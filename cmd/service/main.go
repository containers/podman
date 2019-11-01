package main

import (
	"context"
	"time"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/pkg/serviceapi"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func initConfig() {
	//	we can do more stuff in here.
}

func main() {
	cobra.OnInitialize(initConfig)

	var cancel context.CancelFunc
	_, cancel = context.WithTimeout(context.Background(), time.Duration(50)*time.Millisecond)
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
		log.Panicf("error creating libpod runtime: %s", err)
	}
	defer runtime.DeferredShutdown(false)

	server, _ := serviceapi.NewServer(runtime)
	_ = server.Serve()
	defer server.Shutdown()
}
