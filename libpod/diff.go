package libpod

import (
	"github.com/containers/common/libimage"
	"github.com/containers/podman/v3/libpod/layers"
	"github.com/containers/storage/pkg/archive"
	"github.com/pkg/errors"
)

var initInodes = map[string]bool{
	"/dev":               true,
	"/etc/hostname":      true,
	"/etc/hosts":         true,
	"/etc/resolv.conf":   true,
	"/proc":              true,
	"/run":               true,
	"/run/.containerenv": true,
	"/run/secrets":       true,
	"/sys":               true,
	"/etc/mtab":          true,
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
			if initInodes[c.Path] {
				continue
			}
			rchanges = append(rchanges, c)
		}
	}
	return rchanges, err
}

// GetLayerID gets a full layer id given a full or partial id
// If the id matches a container or image, the id of the top layer is returned
// If the id matches a layer, the top layer id is returned
func (r *Runtime) getLayerID(id string) (string, error) {
	var toLayer string
	toImage, _, err := r.libimageRuntime.LookupImage(id, &libimage.LookupImageOptions{IgnorePlatform: true})
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
