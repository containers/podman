package tunnel

import (
	"context"
	"errors"

	"github.com/containers/podman/v5/pkg/domain/entities"
)

var notImplementedErr = errors.New("not implemented for the remote Podman client")

func (ic *ContainerEngine) QuadletInstall(ctx context.Context, pathsOrURLs []string, options entities.QuadletInstallOptions) (*entities.QuadletInstallReport, error) {
	return nil, notImplementedErr
}

func (ic *ContainerEngine) QuadletList(ctx context.Context, options entities.QuadletListOptions) ([]*entities.ListQuadlet, error) {
	return nil, notImplementedErr
}

func (ic *ContainerEngine) QuadletPrint(ctx context.Context, quadlet string) (string, error) {
	return "", notImplementedErr
}

func (ic *ContainerEngine) QuadletRemove(ctx context.Context, quadlets []string, options entities.QuadletRemoveOptions) (*entities.QuadletRemoveReport, error) {
	return nil, notImplementedErr
}
