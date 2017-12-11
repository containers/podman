// +build containers_image_storage_stub

package alltransports

import "github.com/containers/image/transports"

func init() {
	transports.Register(transports.NewStubTransport("containers-storage"))
}
