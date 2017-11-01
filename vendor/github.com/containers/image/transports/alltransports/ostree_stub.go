// +build containers_image_ostree_stub

package alltransports

import "github.com/containers/image/transports"

func init() {
	transports.Register(transports.NewStubTransport("ostree"))
}
