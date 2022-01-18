package abi

import (
	"context"

	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/pkg/domain/entities"
)

func (ic *ContainerEngine) Events(ctx context.Context, opts entities.EventsOptions) error {
	readOpts := events.ReadOptions{FromStart: opts.FromStart, Stream: opts.Stream, Filters: opts.Filter, EventChannel: opts.EventChan, Since: opts.Since, Until: opts.Until}
	return ic.Libpod.Events(ctx, readOpts)
}
