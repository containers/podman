package image

// GetPruneImages returns a slice of images that have no names/unused
func (ir *Runtime) GetPruneImages() ([]*Image, error) {
	var (
		unamedImages []*Image
	)
	allImages, err := ir.GetImages()
	if err != nil {
		return nil, err
	}
	for _, i := range allImages {
		if len(i.Names()) == 0 {
			unamedImages = append(unamedImages, i)
			continue
		}
		containers, err := i.Containers()
		if err != nil {
			return nil, err
		}
		if len(containers) < 1 {
			unamedImages = append(unamedImages, i)
		}
	}
	return unamedImages, nil
}
