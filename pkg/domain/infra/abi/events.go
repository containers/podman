//+build ABISupport

package abi

import (
	"context"

	"github.com/containers/libpod/libpod/events"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/sirupsen/logrus"
)

func (ic *ContainerEngine) Events(ctx context.Context, opts entities.EventsOptions) error {
	readOpts := events.ReadOptions{FromStart: opts.FromStart, Stream: opts.Stream, Filters: opts.Filter, EventChannel: opts.EventChan, Since: opts.Since, Until: opts.Until}
	err := ic.Libpod.Events(readOpts)
	logrus.Error(err)
	return err
}
