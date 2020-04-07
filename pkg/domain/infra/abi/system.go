// +build ABISupport

package abi

import (
	"context"

	"github.com/containers/libpod/libpod/define"
)

func (ic *ContainerEngine) Info(ctx context.Context) (*define.Info, error) {
	return ic.Libpod.Info()
}
