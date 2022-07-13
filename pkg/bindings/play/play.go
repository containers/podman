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

func Down(ctx context.Context, path string) (*entities.PlayKubeReport, error) {
	return kube.Down(ctx, path)
}

func DownWithBody(ctx context.Context, body io.Reader) (*entities.PlayKubeReport, error) {
	return kube.DownWithBody(ctx, body)
}
