// +build !containers_image_storage_stub

package alltransports

import (
	// Register the storage transport
	_ "github.com/containers/image/v5/storage"
)
