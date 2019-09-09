package shared

import (
	"context"

	"github.com/containers/libpod/libpod"
)

// Remove given set of volumes
func SharedRemoveVolumes(ctx context.Context, runtime *libpod.Runtime, vols []string, all, force bool) ([]string, map[string]error, error) {
	var (
		toRemove []*libpod.Volume
		success  []string
		failed   map[string]error
	)

	failed = make(map[string]error)

	if all {
		vols, err := runtime.Volumes()
		if err != nil {
			return nil, nil, err
		}
		toRemove = vols
	} else {
		for _, v := range vols {
			vol, err := runtime.LookupVolume(v)
			if err != nil {
				failed[v] = err
				continue
			}
			toRemove = append(toRemove, vol)
		}
	}

	// We could parallelize this, but I haven't heard anyone complain about
	// performance here yet, so hold off.
	for _, vol := range toRemove {
		if err := runtime.RemoveVolume(ctx, vol, force); err != nil {
			failed[vol.Name()] = err
			continue
		}
		success = append(success, vol.Name())
	}

	return success, failed, nil
}
