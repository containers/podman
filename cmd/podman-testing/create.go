//go:build !remote

package main

import (
	"fmt"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/containers/podman/v5/internal/domain/entities"
	"github.com/spf13/cobra"
)

var (
	createStorageLayerDescription = `Create an unmanaged layer in local storage.`
	createStorageLayerCmd         = &cobra.Command{
		Use:               "create-storage-layer [options]",
		Args:              validate.NoArgs,
		Short:             "Create an unmanaged layer",
		Long:              createStorageLayerDescription,
		RunE:              createStorageLayer,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman testing create-storage-layer`,
	}

	createStorageLayerOpts entities.CreateStorageLayerOptions

	createLayerDescription = `Create an unused layer in local storage.`
	createLayerCmd         = &cobra.Command{
		Use:               "create-layer [options]",
		Args:              validate.NoArgs,
		Short:             "Create an unused layer",
		Long:              createLayerDescription,
		RunE:              createLayer,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman testing create-layer`,
	}

	createLayerOpts entities.CreateLayerOptions

	createImageDescription = `Create an image in local storage.`
	createImageCmd         = &cobra.Command{
		Use:               "create-image [options]",
		Args:              validate.NoArgs,
		Short:             "Create an image",
		Long:              createImageDescription,
		RunE:              createImage,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman testing create-image`,
	}

	createImageOpts entities.CreateImageOptions

	createContainerDescription = `Create a container in local storage.`
	createContainerCmd         = &cobra.Command{
		Use:               "create-container [options]",
		Args:              validate.NoArgs,
		Short:             "Create a container",
		Long:              createContainerDescription,
		RunE:              createContainer,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman testing create-container`,
	}

	createContainerOpts entities.CreateContainerOptions
)

func init() {
	mainCmd.AddCommand(createStorageLayerCmd)
	flags := createStorageLayerCmd.Flags()
	flags.StringVarP(&createStorageLayerOpts.ID, "id", "i", "", "ID to assign the new layer (default random)")
	flags.StringVarP(&createStorageLayerOpts.Parent, "parent", "p", "", "ID of parent of new layer (default none)")

	mainCmd.AddCommand(createLayerCmd)
	flags = createLayerCmd.Flags()
	flags.StringVarP(&createLayerOpts.ID, "id", "i", "", "ID to assign the new layer (default random)")
	flags.StringVarP(&createLayerOpts.Parent, "parent", "p", "", "ID of parent of new layer (default none)")

	mainCmd.AddCommand(createImageCmd)
	flags = createImageCmd.Flags()
	flags.StringVarP(&createImageOpts.ID, "id", "i", "", "ID to assign the new image (default random)")
	flags.StringVarP(&createImageOpts.Layer, "layer", "l", "", "ID of image's main layer (default none)")

	mainCmd.AddCommand(createContainerCmd)
	flags = createContainerCmd.Flags()
	flags.StringVarP(&createContainerOpts.ID, "id", "i", "", "ID to assign the new container (default random)")
	flags.StringVarP(&createContainerOpts.Image, "image", "b", "", "ID of containers's base image (default none)")
	flags.StringVarP(&createContainerOpts.Layer, "layer", "l", "", "ID of containers's read-write layer (default none)")
}

func createStorageLayer(cmd *cobra.Command, args []string) error {
	results, err := testingEngine.CreateStorageLayer(mainContext, createStorageLayerOpts)
	if err != nil {
		return err
	}

	fmt.Println(results.ID)
	return nil
}

func createLayer(cmd *cobra.Command, args []string) error {
	results, err := testingEngine.CreateLayer(mainContext, createLayerOpts)
	if err != nil {
		return err
	}

	fmt.Println(results.ID)
	return nil
}

func createImage(cmd *cobra.Command, args []string) error {
	results, err := testingEngine.CreateImage(mainContext, createImageOpts)
	if err != nil {
		return err
	}

	fmt.Println(results.ID)
	return nil
}

func createContainer(cmd *cobra.Command, args []string) error {
	results, err := testingEngine.CreateContainer(mainContext, createContainerOpts)
	if err != nil {
		return err
	}

	fmt.Println(results.ID)
	return nil
}
