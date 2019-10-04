// +build containers_image_docker_daemon_stub

package alltransports

import "github.com/containers/image/v4/transports"

func init() {
	transports.Register(transports.NewStubTransport("docker-daemon"))
}
