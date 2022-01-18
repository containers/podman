package tunnel

import (
	"context"
	"strings"

	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/pkg/bindings/system"
	"github.com/containers/podman/v4/pkg/domain/entities"

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
	options := new(system.EventsOptions).WithFilters(filters).WithSince(opts.Since).WithStream(opts.Stream).WithUntil(opts.Until)
	return system.Events(ic.ClientCtx, binChan, nil, options)
}

// GetLastContainerEvent takes a container name or ID and an event status and returns
// the last occurrence of the container event
func (ic *ContainerEngine) GetLastContainerEvent(ctx context.Context, nameOrID string, containerEvent events.Status) (*events.Event, error) {
	// check to make sure the event.Status is valid
	if _, err := events.StringToStatus(containerEvent.String()); err != nil {
		return nil, err
	}
	var event events.Event
	return &event, nil

	/*
		        FIXME: We need new bindings for this section
			filters := []string{
				fmt.Sprintf("container=%s", nameOrID),
				fmt.Sprintf("event=%s", containerEvent),
				"type=container",
			}

			containerEvents, err := system.GetEvents(ctx, entities.EventsOptions{Filter: filters})
			if err != nil {
				return nil, err
			}
			if len(containerEvents) < 1 {
				return nil, errors.Wrapf(events.ErrEventNotFound, "%s not found", containerEvent.String())
			}
			// return the last element in the slice
			return containerEvents[len(containerEvents)-1], nil
	*/
}
