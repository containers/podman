//go:build !remote && (linux || freebsd)

package libpod

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/podman/v6/libpod/events"
)

func TestWaitForNextExitEventStreamClosed(t *testing.T) {
	readErr := errors.New("bad event")
	tests := []struct {
		name    string
		results []events.ReadResult
		wantErr string
	}{
		{"without events", nil, "event stream closed before the container exited"},
		{"after read error", []events.ReadResult{{Error: readErr}}, readErr.Error()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := make(chan events.ReadResult, len(tt.results))
			for _, r := range tt.results {
				ch <- r
			}
			close(ch)

			code, err := waitForNextExitEvent(ch)
			assert.EqualError(t, err, tt.wantErr)
			assert.Equal(t, int32(-1), code)
		})
	}
}
