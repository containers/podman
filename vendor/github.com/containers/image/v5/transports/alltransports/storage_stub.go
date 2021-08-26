//go:build containers_image_storage_stub
// +build containers_image_storage_stub

package alltransports

import "github.com/containers/image/v5/transports"

func init() {
	transports.Register(transports.NewStubTransport("containers-storage"))
}
