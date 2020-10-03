package libpod

import (
	"io"

	"github.com/containers/podman/v2/libpod/layers"
	"github.com/containers/storage/pkg/archive"
	"github.com/pkg/errors"
)

var containerMounts = map[string]bool{
	"/dev":               true,
	"/etc/hostname":      true,
	"/etc/hosts":         true,
	"/etc/resolv.conf":   true,
	"/proc":              true,
	"/run":               true,
	"/run/.containerenv": true,
	"/run/secrets":       true,
	"/sys":               true,
}

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
	var rchanges []archive.Change
	changes, err := r.store.Changes(fromLayer, toLayer)
	if err == nil {
		for _, c := range changes {
			if containerMounts[c.Path] {
				continue
			}
			rchanges = append(rchanges, c)
		}
	}
	return rchanges, err
}

// ApplyDiffTarStream applies the changes stored in 'diff' to the layer 'to'
func (r *Runtime) ApplyDiffTarStream(to string, diff io.Reader) error {
	toLayer, err := r.getLayerID(to)
	if err != nil {
		return err
	}
	_, err = r.store.ApplyDiff(toLayer, diff)
	return err
}

// GetLayerID gets a full layer id given a full or partial id
// If the id matches a container or image, the id of the top layer is returned
// If the id matches a layer, the top layer id is returned
func (r *Runtime) getLayerID(id string) (string, error) {
	var toLayer string
	toImage, err := r.imageRuntime.NewFromLocal(id)
	if err == nil {
		return toImage.TopLayer(), nil
	}

	targetID, err := r.store.Lookup(id)
	if err != nil {
		targetID = id
	}
	toCtr, err := r.store.Container(targetID)
	if err != nil {
		toLayer, err = layers.FullID(r.store, targetID)
		if err != nil {
			return "", errors.Errorf("layer, image, or container %s does not exist", id)
		}
	} else {
		toLayer = toCtr.LayerID
	}
	return toLayer, nil
}
