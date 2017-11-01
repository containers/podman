package libpod

import (
	"github.com/containers/storage/pkg/archive"
	"github.com/kubernetes-incubator/cri-o/libpod/layers"
	"github.com/pkg/errors"
)

// GetDiff returns the differences between the two images, layers, or containers
func (r *Runtime) GetDiff(from, to string) ([]archive.Change, error) {
	toLayer, err := r.getLayerID(to)
	if err != nil {
		return nil, err
	}
	fromLayer := ""
	if from != "" {
		fromLayer, err = r.getLayerID(from)
		if err != nil {
			return nil, err
		}
	}
	return r.store.Changes(fromLayer, toLayer)
}

// GetLayerID gets a full layer id given a full or partial id
// If the id matches a container or image, the id of the top layer is returned
// If the id matches a layer, the top layer id is returned
func (r *Runtime) getLayerID(id string) (string, error) {
	var toLayer string
	toImage, err := r.GetImage(id)
	if err != nil {
		toCtr, err := r.store.Container(id)
		if err != nil {
			toLayer, err = layers.FullID(r.store, id)
			if err != nil {
				return "", errors.Errorf("layer, image, or container %s does not exist", id)
			}
		} else {
			toLayer = toCtr.LayerID
		}
	} else {
		toLayer = toImage.TopLayer
	}
	return toLayer, nil
}

func (r *Runtime) getLayerParent(layerID string) (string, error) {
	layer, err := r.store.Layer(layerID)
	if err != nil {
		return "", err
	}
	return layer.Parent, nil
}
