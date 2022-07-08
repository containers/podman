package exec

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"time"

	"github.com/davecgh/go-spew/spew"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/sirupsen/logrus"
)

var spewConfig = spew.ConfigState{
	Indent:                  " ",
	DisablePointerAddresses: true,
	DisableCapacities:       true,
	SortKeys:                true,
}

// RuntimeConfigFilter calls a series of hooks.  But instead of
// passing container state on their standard input,
// RuntimeConfigFilter passes the proposed runtime configuration (and
// reads back a possibly-altered form from their standard output).
func RuntimeConfigFilter(ctx context.Context, hooks []spec.Hook, config *spec.Spec, postKillTimeout time.Duration) (hookErr, err error) {
	data, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	for i, hook := range hooks {
		hook := hook
		var stdout bytes.Buffer
		hookErr, err = Run(ctx, &hook, data, &stdout, nil, postKillTimeout)
		if err != nil {
			return hookErr, err
		}

		data = stdout.Bytes()
		var newConfig spec.Spec
		err = json.Unmarshal(data, &newConfig)
		if err != nil {
			logrus.Debugf("invalid JSON from config-filter hook %d:\n%s", i, string(data))
			return nil, errors.Wrapf(err, "unmarshal output from config-filter hook %d", i)
		}

		if !reflect.DeepEqual(config, &newConfig) {
			oldConfig := spewConfig.Sdump(config)
			newConfig := spewConfig.Sdump(&newConfig)
			diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
				A:        difflib.SplitLines(oldConfig),
				B:        difflib.SplitLines(newConfig),
				FromFile: "Old",
				FromDate: "",
				ToFile:   "New",
				ToDate:   "",
				Context:  1,
			})
			if err == nil {
				logrus.Debugf("precreate hook %d made configuration changes:\n%s", i, diff)
			} else {
				logrus.Warnf("Precreate hook %d made configuration changes, but we could not compute a diff: %v", i, err)
			}
		}

		*config = newConfig
	}

	return nil, nil
}
