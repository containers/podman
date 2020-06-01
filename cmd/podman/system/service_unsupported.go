// +build !ABISupport,!remote

package system

import (
	"errors"

	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/pflag"
)

func restService(opts entities.ServiceOptions, flags *pflag.FlagSet, cfg *entities.PodmanConfig) error {
	return errors.New("not supported")
}
