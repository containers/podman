//go:build !remote

package main

import (
	"errors"
	"os"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/containers/podman/v5/internal/domain/entities"
	"github.com/spf13/cobra"
)

var (
	populateLayerDescription = `Populate a layer in local storage.`
	populateLayerCmd         = &cobra.Command{
		Use:               "populate-layer [options]",
		Args:              validate.NoArgs,
		Short:             "Populate a layer",
		Long:              populateLayerDescription,
		RunE:              populateLayer,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman testing populate-layer`,
	}

	populateLayerOpts entities.PopulateLayerOptions
	populateLayerFile string

	modifyLayerDescription = `Modify a layer in local storage, corrupting it.`
	modifyLayerCmd         = &cobra.Command{
		Use:               "modify-layer [options]",
		Args:              validate.NoArgs,
		Short:             "Modify the contents of a layer",
		Long:              modifyLayerDescription,
		RunE:              modifyLayer,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman testing modify-layer`,
	}

	modifyLayerOpts entities.ModifyLayerOptions
	modifyLayerFile string
)

func init() {
	mainCmd.AddCommand(populateLayerCmd)
	flags := populateLayerCmd.Flags()
	flags.StringVarP(&populateLayerOpts.ID, "layer", "l", "", "ID of layer to be populated")
	flags.StringVarP(&populateLayerFile, "file", "f", "", "archive of contents to extract in layer")

	mainCmd.AddCommand(modifyLayerCmd)
	flags = modifyLayerCmd.Flags()
	flags.StringVarP(&modifyLayerOpts.ID, "layer", "l", "", "ID of layer to be modified")
	flags.StringVarP(&modifyLayerFile, "file", "f", "", "archive of contents to extract over layer")
}

func populateLayer(cmd *cobra.Command, args []string) error {
	if populateLayerOpts.ID == "" {
		return errors.New("layer ID not specified")
	}
	if populateLayerFile == "" {
		return errors.New("layer contents file not specified")
	}
	buf, err := os.ReadFile(populateLayerFile)
	if err != nil {
		return err
	}
	populateLayerOpts.ContentsArchive = buf
	_, err = testingEngine.PopulateLayer(mainContext, populateLayerOpts)
	if err != nil {
		return err
	}
	return nil
}

func modifyLayer(cmd *cobra.Command, args []string) error {
	if modifyLayerOpts.ID == "" {
		return errors.New("layer ID not specified")
	}
	if modifyLayerFile == "" {
		return errors.New("layer contents file not specified")
	}
	buf, err := os.ReadFile(modifyLayerFile)
	if err != nil {
		return err
	}
	modifyLayerOpts.ContentsArchive = buf
	_, err = testingEngine.ModifyLayer(mainContext, modifyLayerOpts)
	if err != nil {
		return err
	}
	return nil
}
