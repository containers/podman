package main

import (
	"os"
	"reflect"

	_ "github.com/containers/libpod/cmd/podmanV2/containers"
	_ "github.com/containers/libpod/cmd/podmanV2/healthcheck"
	_ "github.com/containers/libpod/cmd/podmanV2/images"
	_ "github.com/containers/libpod/cmd/podmanV2/networks"
	_ "github.com/containers/libpod/cmd/podmanV2/pods"
	"github.com/containers/libpod/cmd/podmanV2/registry"
	_ "github.com/containers/libpod/cmd/podmanV2/system"
	_ "github.com/containers/libpod/cmd/podmanV2/volumes"
	"github.com/containers/storage/pkg/reexec"
)

func init() {
	// This is the bootstrap configuration, if user gives
	// CLI flags parts of this configuration may be overwritten
	registry.PodmanOptions = registry.NewPodmanConfig()
}

func main() {
	if reexec.Init() {
		// We were invoked with a different argv[0] indicating that we
		// had a specific job to do as a subprocess, and it's done.
		return
	}
	for _, c := range registry.Commands {
		if Contains(registry.PodmanOptions.EngineMode, c.Mode) {
			parent := rootCmd
			if c.Parent != nil {
				parent = c.Parent
			}
			parent.AddCommand(c.Command)
		}
	}

	Execute()
	os.Exit(0)
}

func Contains(item interface{}, slice interface{}) bool {
	s := reflect.ValueOf(slice)

	switch s.Kind() {
	case reflect.Array:
		fallthrough
	case reflect.Slice:
		break
	default:
		return false
	}

	for i := 0; i < s.Len(); i++ {
		if s.Index(i).Interface() == item {
			return true
		}
	}
	return false
}
