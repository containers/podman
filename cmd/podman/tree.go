package main

import (
	"context"
	"fmt"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	middleItem   = "├── "
	continueItem = "│   "
	lastItem     = "└── "
)

var (
	treeCommand cliconfig.TreeValues

	treeDescription = "Prints layer hierarchy of an image in a tree format"
	_treeCommand    = &cobra.Command{
		Use:   "tree [flags] IMAGE",
		Short: treeDescription,
		Long:  treeDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			treeCommand.InputArgs = args
			treeCommand.GlobalFlags = MainGlobalOpts
			treeCommand.Remote = remoteclient
			return treeCmd(&treeCommand)
		},
		Example: "podman image tree alpine:latest",
	}
)

func init() {
	treeCommand.Command = _treeCommand
	treeCommand.SetUsageTemplate(UsageTemplate())
	treeCommand.Flags().BoolVar(&treeCommand.WhatRequires, "whatrequires", false, "Show all child images and layers of the specified image")
}

func treeCmd(c *cliconfig.TreeValues) error {
	args := c.InputArgs
	if len(args) == 0 {
		return errors.Errorf("an image name must be specified")
	}
	if len(args) > 1 {
		return errors.Errorf("you must provide at most 1 argument")
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)
	imageInfo, layerInfoMap, img, err := runtime.Tree(c)
	if err != nil {
		return err
	}
	return printTree(imageInfo, layerInfoMap, img, c.WhatRequires)
}

func printTree(imageInfo *image.InfoImage, layerInfoMap map[string]*image.LayerInfo, img *adapter.ContainerImage, whatRequires bool) error {
	size, err := img.Size(context.Background())
	if err != nil {
		return err
	}

	fmt.Printf("Image ID: %s\n", imageInfo.ID[:12])
	fmt.Printf("Tags:\t %s\n", imageInfo.Tags)
	fmt.Printf("Size:\t %v\n", units.HumanSizeWithPrecision(float64(*size), 4))
	if img.TopLayer() != "" {
		fmt.Printf("Image Layers\n")
	} else {
		fmt.Printf("No Image Layers\n")
	}

	if !whatRequires {
		// fill imageInfo with layers associated with image.
		// the layers will be filled such that
		// (Start)RootLayer->...intermediate Parent Layer(s)-> TopLayer(End)
		// Build output from imageInfo into buffer
		printImageHierarchy(imageInfo)

	} else {
		// fill imageInfo with layers associated with image.
		// the layers will be filled such that
		// (Start)TopLayer->...intermediate Child Layer(s)-> Child TopLayer(End)
		//             (Forks)... intermediate Child Layer(s) -> Child Top Layer(End)
		return printImageChildren(layerInfoMap, img.TopLayer(), "", true)
	}
	return nil
}

// Stores all children layers which are created using given Image.
// Layers are stored as follows
// (Start)TopLayer->...intermediate Child Layer(s)-> Child TopLayer(End)
//             (Forks)... intermediate Child Layer(s) -> Child Top Layer(End)
func printImageChildren(layerMap map[string]*image.LayerInfo, layerID string, prefix string, last bool) error {
	if layerID == "" {
		return nil
	}
	ll, ok := layerMap[layerID]
	if !ok {
		return fmt.Errorf("lookup error: layerid  %s, not found", layerID)
	}
	fmt.Printf(prefix)

	//initialize intend with middleItem to reduce middleItem checks.
	intend := middleItem
	if !last {
		// add continueItem i.e. '|' for next iteration prefix
		prefix = prefix + continueItem
	} else if len(ll.ChildID) > 1 || len(ll.ChildID) == 0 {
		// The above condition ensure, alignment happens for node, which has more then 1 childern.
		// If node is last in printing hierarchy, it should not be printed as middleItem i.e. ├──
		intend = lastItem
		prefix = prefix + " "
	}

	var tags string
	if len(ll.RepoTags) > 0 {
		tags = fmt.Sprintf(" Top Layer of: %s", ll.RepoTags)
	}
	fmt.Printf("%sID: %s Size: %7v%s\n", intend, ll.ID[:12], units.HumanSizeWithPrecision(float64(ll.Size), 4), tags)
	for count, childID := range ll.ChildID {
		if err := printImageChildren(layerMap, childID, prefix, count == len(ll.ChildID)-1); err != nil {
			return err
		}
	}
	return nil
}

// prints the layers info of image
func printImageHierarchy(imageInfo *image.InfoImage) {
	for count, l := range imageInfo.Layers {
		var tags string
		intend := middleItem
		if len(l.RepoTags) > 0 {
			tags = fmt.Sprintf(" Top Layer of: %s", l.RepoTags)
		}
		if count == len(imageInfo.Layers)-1 {
			intend = lastItem
		}
		fmt.Printf("%s ID: %s Size: %7v%s\n", intend, l.ID[:12], units.HumanSizeWithPrecision(float64(l.Size), 4), tags)
	}
}
