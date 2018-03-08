package image

import (
	"github.com/containers/image/docker/reference"
	"github.com/containers/storage"
	"github.com/pkg/errors"
)

func getTags(nameInput string) (reference.NamedTagged, bool, error) {
	inputRef, err := reference.Parse(nameInput)
	if err != nil {
		return nil, false, errors.Wrapf(err, "unable to obtain tag from input name")
	}
	tagged, isTagged := inputRef.(reference.NamedTagged)

	return tagged, isTagged, nil
}

// findImageInRepotags takes an imageParts struct and searches images' repotags for
// a match on name:tag
func findImageInRepotags(search imageParts, images []*storage.Image) (*storage.Image, error) {
	var results []*storage.Image
	for _, image := range images {
		for _, name := range image.Names {
			d, err := decompose(name)
			// if we get an error, ignore and keep going
			if err != nil {
				continue
			}
			if d.name == search.name && d.tag == search.tag {
				results = append(results, image)
				break
			}
		}
	}
	if len(results) == 0 {
		return &storage.Image{}, errors.Errorf("unable to find a name and tag match for %s in repotags", search)
	} else if len(results) > 1 {
		return &storage.Image{}, errors.Errorf("found multiple name and tag matches for %s in repotags", search)
	}
	return results[0], nil
}
