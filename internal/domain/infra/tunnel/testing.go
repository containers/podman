//go:build !remote

package tunnel

import (
	"context"
	"syscall"

	"github.com/containers/podman/v5/internal/domain/entities"
)

type TestingEngine struct {
	ClientCtx context.Context
}

func (te *TestingEngine) CreateStorageLayer(ctx context.Context, opts entities.CreateStorageLayerOptions) (*entities.CreateStorageLayerReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) CreateLayer(ctx context.Context, opts entities.CreateLayerOptions) (*entities.CreateLayerReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) CreateLayerData(ctx context.Context, opts entities.CreateLayerDataOptions) (*entities.CreateLayerDataReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) ModifyLayer(ctx context.Context, opts entities.ModifyLayerOptions) (*entities.ModifyLayerReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) PopulateLayer(ctx context.Context, opts entities.PopulateLayerOptions) (*entities.PopulateLayerReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) RemoveStorageLayer(ctx context.Context, opts entities.RemoveStorageLayerOptions) (*entities.RemoveStorageLayerReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) CreateImage(ctx context.Context, opts entities.CreateImageOptions) (*entities.CreateImageReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) CreateImageData(ctx context.Context, opts entities.CreateImageDataOptions) (*entities.CreateImageDataReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) RemoveLayer(ctx context.Context, opts entities.RemoveLayerOptions) (*entities.RemoveLayerReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) RemoveImage(ctx context.Context, opts entities.RemoveImageOptions) (*entities.RemoveImageReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) RemoveContainer(ctx context.Context, opts entities.RemoveContainerOptions) (*entities.RemoveContainerReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) CreateContainer(ctx context.Context, opts entities.CreateContainerOptions) (*entities.CreateContainerReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) CreateContainerData(ctx context.Context, opts entities.CreateContainerDataOptions) (*entities.CreateContainerDataReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) RemoveLayerData(ctx context.Context, opts entities.RemoveLayerDataOptions) (*entities.RemoveLayerDataReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) RemoveImageData(ctx context.Context, opts entities.RemoveImageDataOptions) (*entities.RemoveImageDataReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) RemoveContainerData(ctx context.Context, opts entities.RemoveContainerDataOptions) (*entities.RemoveContainerDataReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) ModifyLayerData(ctx context.Context, opts entities.ModifyLayerDataOptions) (*entities.ModifyLayerDataReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) ModifyImageData(ctx context.Context, opts entities.ModifyImageDataOptions) (*entities.ModifyImageDataReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) ModifyContainerData(ctx context.Context, opts entities.ModifyContainerDataOptions) (*entities.ModifyContainerDataReport, error) {
	return nil, syscall.ENOSYS
}
