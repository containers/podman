export DOCKER_HOST="unix://$(podman info -f "{{.Host.RemoteSocket.Path}}")"
