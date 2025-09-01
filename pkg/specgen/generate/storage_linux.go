//go:build !remote

package generate

import (
	"context"

	"go.podman.io/common/libimage"
)

func imageRunPath(ctx context.Context, img *libimage.Image) (string, error) {
	return "/run", nil
}
