package tunnel

import (
	"context"
	"errors"

	"github.com/containers/podman/v4/pkg/domain/entities"
)

func (ir *ImageEngine) ShowTrust(ctx context.Context, args []string, options entities.ShowTrustOptions) (*entities.ShowTrustReport, error) {
	return nil, errors.New("not implemented")
}

func (ir *ImageEngine) SetTrust(ctx context.Context, args []string, options entities.SetTrustOptions) error {
	return errors.New("not implemented")
}
