// +build !ostree_repos !linux

package alltransports

import "github.com/containers/image/transports"

func init() {
	transports.Register(transports.NewStubTransport("ostree"))
}
