// +build !containers_image_docker_daemon_stub

package alltransports

import (
	// Register the docker-daemon transport
	_ "github.com/containers/image/v5/docker/daemon"
)
