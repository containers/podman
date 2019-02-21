// +build ostree_repos,linux

package alltransports

import (
	// Register the ostree transport
	_ "github.com/containers/image/ostree"
)
