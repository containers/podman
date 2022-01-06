package util

import (
	"github.com/containers/common/libimage"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
)

// LookupImage returns *Image to corresponding imagename or id
func LookupImage(ctx *types.SystemContext, store storage.Store, image string) (*libimage.Image, error) {
	systemContext := ctx
	if systemContext == nil {
		systemContext = &types.SystemContext{}
	}
	runtime, err := libimage.RuntimeFromStore(store, &libimage.RuntimeOptions{SystemContext: systemContext})
	if err != nil {
		return nil, err
	}
	localImage, _, err := runtime.LookupImage(image, nil)
	if err != nil {
		return nil, err
	}
	return localImage, nil
}
