package tunnel

import (
	"context"
	"fmt"
	"strings"

	"github.com/containers/podman/v5/libpod/events"
	"github.com/containers/podman/v5/pkg/bindings/system"
	"github.com/containers/podman/v5/pkg/domain/entities"
)

func (ic *ContainerEngine) Events(ctx context.Context, opts entities.EventsOptions) error {
	filters := make(map[string][]string)
	if len(opts.Filter) > 0 {
		for _, filter := range opts.Filter {
			split := strings.Split(filter, "=")
			if len(split) < 2 {
				return fmt.Errorf("invalid filter %q", filter)
			}
			filters[split[0]] = append(filters[split[0]], strings.Join(split[1:], "="))
		}
	}
	binChan := make(chan entities.Event)
	go func() {
		for e := range binChan {
			opts.EventChan <- events.ReadResult{Event: entities.ConvertToLibpodEvent(e)}
		}
		close(opts.EventChan)
	}()
	options := new(system.EventsOptions).WithFilters(filters).WithSince(opts.Since).WithStream(opts.Stream).WithUntil(opts.Until)
	return system.Events(ic.ClientCtx, binChan, nil, options)
}
