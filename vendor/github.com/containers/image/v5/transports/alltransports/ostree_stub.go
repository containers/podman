// +build !containers_image_ostree !linux

package alltransports

import "github.com/containers/image/v5/transports"

func init() {
	transports.Register(transports.NewStubTransport("ostree"))
}
