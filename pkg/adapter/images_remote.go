// +build remoteclient

package adapter

import (
	"context"
	"encoding/json"

	"github.com/containers/libpod/pkg/inspect"
	iopodman "github.com/containers/libpod/pkg/varlink"
)

// Inspect returns returns an ImageData struct from over a varlink connection
func (i *ContainerImage) Inspect(ctx context.Context) (*inspect.ImageData, error) {
	reply, err := iopodman.InspectImage().Call(i.Runtime.Conn, i.ID())
	if err != nil {
		return nil, err
	}
	data := inspect.ImageData{}
	if err := json.Unmarshal([]byte(reply), &data); err != nil {
		return nil, err
	}
	return &data, nil
}
