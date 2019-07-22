// +build !containers_image_ostree !linux

package alltransports

import "github.com/containers/image/transports"

func init() {
	transports.Register(transports.NewStubTransport("ostree"))
}
