// +build remoteclient

package adapter

import (
	"context"
	"encoding/json"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	iopodman "github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/inspect"
	"github.com/pkg/errors"
)

// Inspect returns returns an ImageData struct from over a varlink connection
func (i *ContainerImage) Inspect(ctx context.Context) (*inspect.ImageData, error) {
	reply, err := iopodman.InspectImage().Call(i.Runtime.Conn, i.ID())
	if err != nil {
		return nil, err
	}
	data := inspect.ImageData{}
	if err := json.Unmarshal([]byte(reply), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// Tree ...
func (r *LocalRuntime) Tree(c *cliconfig.TreeValues) (*image.InfoImage, map[string]*image.LayerInfo, *ContainerImage, error) {
	layerInfoMap := make(map[string]*image.LayerInfo)
	imageInfo := &image.InfoImage{}

	img, err := r.NewImageFromLocal(c.InputArgs[0])
	if err != nil {
		return nil, nil, nil, err
	}

	reply, err := iopodman.GetLayersMapWithImageInfo().Call(r.Conn)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to obtain image layers")
	}
	if err := json.Unmarshal([]byte(reply), &layerInfoMap); err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to unmarshal image layers")
	}

	reply, err = iopodman.BuildImageHierarchyMap().Call(r.Conn, c.InputArgs[0])
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to get build image map")
	}
	if err := json.Unmarshal([]byte(reply), imageInfo); err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to unmarshal build image map")
	}

	return imageInfo, layerInfoMap, img, nil
}

// UmountImage removes container(s) based on CLI inputs.
func (r *LocalRuntime) UmountImage(cli *cliconfig.UmountValues) ([]string, map[string]error, error) {
	var (
		ok       = []string{}
		failures = map[string]error{}
	)

	_, err := iopodman.UnmountImage().Call(r.Conn, cli.InputArgs[0], cli.Force)
	if err != nil {
		return ok, failures, err
	}
	ok = append(ok, cli.InputArgs[1])
	return ok, failures, nil
}
