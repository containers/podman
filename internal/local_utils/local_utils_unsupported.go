//go:build !remote || (!amd64 && !arm64)

package local_utils

import (
	"context"

	"github.com/sirupsen/logrus"
)

func CheckPathOnRunningMachine(ctx context.Context, path string) (*LocalAPIMap, bool) {
	logrus.Debug("CheckPathOnRunningMachine is not supported")
	return nil, false
}
