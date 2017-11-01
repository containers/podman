package main

import (
	"os"
	"reflect"
	"regexp"
	"strings"

	is "github.com/containers/image/storage"
	"github.com/containers/storage"
	"github.com/fatih/camelcase"
	"github.com/kubernetes-incubator/cri-o/libkpod"
	"github.com/kubernetes-incubator/cri-o/libpod"
	"github.com/kubernetes-incubator/cri-o/server"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	stores = make(map[storage.Store]struct{})
)

func getStore(c *libkpod.Config) (storage.Store, error) {
	options := storage.DefaultStoreOptions
	options.GraphRoot = c.Root
	options.RunRoot = c.RunRoot
	options.GraphDriverName = c.Storage
	options.GraphDriverOptions = c.StorageOptions

	store, err := storage.GetStore(options)
	if err != nil {
		return nil, err
	}
	is.Transport.SetStore(store)
	stores[store] = struct{}{}
	return store, nil
}

func getRuntime(c *cli.Context) (*libpod.Runtime, error) {

	config, err := getConfig(c)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get config")
	}

	options := storage.DefaultStoreOptions
	options.GraphRoot = config.Root
	options.RunRoot = config.RunRoot
	options.GraphDriverName = config.Storage
	options.GraphDriverOptions = config.StorageOptions

	return libpod.NewRuntime(libpod.WithStorageConfig(options))
}

func shutdownStores() {
	for store := range stores {
		if _, err := store.Shutdown(false); err != nil {
			break
		}
	}
}

func getConfig(c *cli.Context) (*libkpod.Config, error) {
	config := libkpod.DefaultConfig()
	var configFile string
	if c.GlobalIsSet("config") {
		configFile = c.GlobalString("config")
	} else if _, err := os.Stat(server.CrioConfigPath); err == nil {
		configFile = server.CrioConfigPath
	}
	// load and merge the configfile from the commandline or use
	// the default crio config file
	if configFile != "" {
		err := config.UpdateFromFile(configFile)
		if err != nil {
			return config, err
		}
	}
	if c.GlobalIsSet("root") {
		config.Root = c.GlobalString("root")
	}
	if c.GlobalIsSet("runroot") {
		config.RunRoot = c.GlobalString("runroot")
	}

	if c.GlobalIsSet("storage-driver") {
		config.Storage = c.GlobalString("storage-driver")
	}
	if c.GlobalIsSet("storage-opt") {
		opts := c.GlobalStringSlice("storage-opt")
		if len(opts) > 0 {
			config.StorageOptions = opts
		}
	}
	if c.GlobalIsSet("runtime") {
		config.Runtime = c.GlobalString("runtime")
	}
	return config, nil
}

func splitCamelCase(src string) string {
	entries := camelcase.Split(src)
	return strings.Join(entries, " ")
}

// validateFlags searches for StringFlags or StringSlice flags that never had
// a value set.  This commonly occurs when the CLI mistakenly takes the next
// option and uses it as a value.
func validateFlags(c *cli.Context, flags []cli.Flag) error {
	for _, flag := range flags {
		switch reflect.TypeOf(flag).String() {
		case "cli.StringSliceFlag":
			{
				f := flag.(cli.StringSliceFlag)
				name := strings.Split(f.Name, ",")
				val := c.StringSlice(name[0])
				for _, v := range val {
					if ok, _ := regexp.MatchString("^-.+", v); ok {
						return errors.Errorf("option --%s requires a value", name[0])
					}
				}
			}
		case "cli.StringFlag":
			{
				f := flag.(cli.StringFlag)
				name := strings.Split(f.Name, ",")
				val := c.String(name[0])
				if ok, _ := regexp.MatchString("^-.+", val); ok {
					return errors.Errorf("option --%s requires a value", name[0])
				}
			}
		}
	}
	return nil
}
