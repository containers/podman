//go:build !amd64 && !arm64

package localapi

import (
	"context"

	"github.com/sirupsen/logrus"
)

func CheckPathOnRunningMachine(ctx context.Context, path string) (*LocalAPIMap, bool) {
	logrus.Debug("CheckPathOnRunningMachine is not supported")
	return nil, false
}
