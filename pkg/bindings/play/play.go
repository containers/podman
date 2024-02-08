package play

import (
	"context"
	"io"

	"github.com/containers/podman/v5/pkg/bindings/kube"
	"github.com/containers/podman/v5/pkg/domain/entities/types"
)

type KubeOptions = kube.PlayOptions

func Kube(ctx context.Context, path string, options *KubeOptions) (*types.PlayKubeReport, error) {
	return kube.Play(ctx, path, options)
}

func KubeWithBody(ctx context.Context, body io.Reader, options *KubeOptions) (*types.PlayKubeReport, error) {
	return kube.PlayWithBody(ctx, body, options)
}

func Down(ctx context.Context, path string, options kube.DownOptions) (*types.PlayKubeReport, error) {
	return kube.Down(ctx, path, options)
}

func DownWithBody(ctx context.Context, body io.Reader, options kube.DownOptions) (*types.PlayKubeReport, error) {
	return kube.DownWithBody(ctx, body, options)
}
