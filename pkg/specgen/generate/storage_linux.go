//go:build !remote

package generate

import (
	"context"

	"github.com/containers/common/libimage"
)

func imageRunPath(ctx context.Context, img *libimage.Image) (string, error) {
	return "/run", nil
}
