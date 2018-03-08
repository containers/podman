// +build containers_image_ostree_stub !linux

package alltransports

import "github.com/containers/image/transports"

func init() {
	transports.Register(transports.NewStubTransport("ostree"))
}
