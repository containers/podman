package images

import (
	"net/url"

	"github.com/containers/podman/v4/pkg/bindings/internal/util"
)

// ToParams formats struct fields to be passed to API service
func (o *ScpOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}
