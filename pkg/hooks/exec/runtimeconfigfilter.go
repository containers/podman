package exec

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// RuntimeConfigFilter calls a series of hooks.  But instead of
// passing container state on their standard input,
// RuntimeConfigFilter passes the proposed runtime configuration (and
// reads back a possibly-altered form from their standard output).
func RuntimeConfigFilter(ctx context.Context, hooks []spec.Hook, config *spec.Spec, postKillTimeout time.Duration) (hookErr, err error) {
	data, err := json.Marshal(config)
	for _, hook := range hooks {
		var stdout bytes.Buffer
		hookErr, err = Run(ctx, &hook, data, &stdout, nil, postKillTimeout)
		if err != nil {
			return hookErr, err
		}

		data = stdout.Bytes()
	}
	err = json.Unmarshal(data, config)
	if err != nil {
		logrus.Debugf("invalid JSON from config-filter hooks:\n%s", string(data))
		return nil, errors.Wrap(err, "unmarshal output from config-filter hooks")
	}

	return nil, nil
}
