package main

import (
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strings"

	_ "github.com/containers/libpod/cmd/podmanV2/containers"
	_ "github.com/containers/libpod/cmd/podmanV2/images"
	_ "github.com/containers/libpod/cmd/podmanV2/networks"
	_ "github.com/containers/libpod/cmd/podmanV2/pods"
	"github.com/containers/libpod/cmd/podmanV2/registry"
	_ "github.com/containers/libpod/cmd/podmanV2/volumes"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	if err := libpod.SetXdgDirs(); err != nil {
		logrus.Errorf(err.Error())
		os.Exit(1)
	}
	initCobra()
}

func initCobra() {
	switch runtime.GOOS {
	case "darwin":
		fallthrough
	case "windows":
		registry.EngineOptions.EngineMode = entities.TunnelMode
	case "linux":
		registry.EngineOptions.EngineMode = entities.ABIMode
	default:
		logrus.Errorf("%s is not a supported OS", runtime.GOOS)
		os.Exit(1)
	}

	// TODO: Is there a Cobra way to "peek" at os.Args?
	for _, v := range os.Args {
		if strings.HasPrefix(v, "--remote") {
			registry.EngineOptions.EngineMode = entities.TunnelMode
		}
	}

	cobra.OnInitialize(func() {})
}

func main() {
	fmt.Fprintf(os.Stderr, "Number of commands: %d\n", len(registry.Commands))
	for _, c := range registry.Commands {
		if Contains(registry.EngineOptions.EngineMode, c.Mode) {
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
