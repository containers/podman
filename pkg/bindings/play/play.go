package play

import (
	"context"
	"io"

	"github.com/containers/podman/v4/pkg/bindings/kube"
	"github.com/containers/podman/v4/pkg/domain/entities"
)

type KubeOptions = kube.PlayOptions

func Kube(ctx context.Context, path string, options *KubeOptions) (*entities.PlayKubeReport, error) {
	return kube.Play(ctx, path, options)
}

func KubeWithBody(ctx context.Context, body io.Reader, options *KubeOptions) (*entities.PlayKubeReport, error) {
	return kube.PlayWithBody(ctx, body, options)
}

func Down(ctx context.Context, path string, options kube.DownOptions) (*entities.PlayKubeReport, error) {
	return kube.Down(ctx, path, options)
}

func DownWithBody(ctx context.Context, body io.Reader, options kube.DownOptions) (*entities.PlayKubeReport, error) {
	return kube.DownWithBody(ctx, body, options)
}
