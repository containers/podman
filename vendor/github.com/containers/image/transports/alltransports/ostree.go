// +build !containers_image_ostree_stub,linux

package alltransports

import (
	// Register the ostree transport
	_ "github.com/containers/image/ostree"
)
