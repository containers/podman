package tunnel

import (
	"context"
	"strings"

	"github.com/containers/libpod/pkg/bindings/system"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
)

func (ic *ContainerEngine) Events(ctx context.Context, opts entities.EventsOptions) error {
	filters := make(map[string][]string)
	if len(opts.Filter) > 0 {
		for _, filter := range opts.Filter {
			split := strings.Split(filter, "=")
			if len(split) < 2 {
				return errors.Errorf("invalid filter %q", filter)
			}
			filters[split[0]] = append(filters[split[0]], strings.Join(split[1:], "="))
		}
	}
	binChan := make(chan entities.Event)
	go func() {
		for e := range binChan {
			opts.EventChan <- entities.ConvertToLibpodEvent(e)
		}
		close(opts.EventChan)
	}()
	return system.Events(ic.ClientCxt, binChan, nil, &opts.Since, &opts.Until, filters, &opts.Stream)
}
