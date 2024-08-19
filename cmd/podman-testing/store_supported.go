//go:build linux && !remote

package main

import (
	"fmt"
	"os"

	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/storage"
	"github.com/containers/storage/types"
)

var (
	globalStore storage.Store
	engineMode  = entities.ABIMode
)

func init() {
	if defaultStoreOptions, err := storage.DefaultStoreOptions(); err == nil {
		globalStorageOptions = defaultStoreOptions
	}
	if storageConf, ok := os.LookupEnv("CONTAINERS_STORAGE_CONF"); ok {
		options := globalStorageOptions
		if types.ReloadConfigurationFileIfNeeded(storageConf, &options) == nil {
			globalStorageOptions = options
		}
	}
	fl := mainCmd.PersistentFlags()
	fl.StringVar(&globalStorageOptions.GraphDriverName, "storage-driver", "", "storage driver used to manage images and containers")
	fl.StringVar(&globalStorageOptions.GraphRoot, "root", "", "where images and containers will be stored")
	fl.StringVar(&globalStorageOptions.RunRoot, "runroot", "", "where volatile state information will be stored")
	fl.StringArrayVar(&globalStorageOptions.GraphDriverOptions, "storage-opt", nil, "storage driver options")
	fl.StringVar(&globalStorageOptions.ImageStore, "imagestore", "", "where to store just some parts of images")
	fl.BoolVar(&globalStorageOptions.TransientStore, "transient-store", false, "enable transient container storage")
}

func storeBefore() error {
	defaultStoreOptions, err := storage.DefaultStoreOptions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "selecting storage options: %v", err)
		return nil
	}
	globalStorageOptions = defaultStoreOptions
	store, err := storage.GetStore(globalStorageOptions)
	if err != nil {
		return err
	}
	globalStore = store
	if podmanConfig.URI != "" {
		engineMode = entities.TunnelMode
	} else {
		engineMode = entities.ABIMode
	}
	return nil
}

func storeAfter() error {
	if globalStore != nil {
		_, err := globalStore.Shutdown(false)
		return err
	}
	return nil
}
