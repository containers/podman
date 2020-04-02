all: podman podman-remote

podman:
	CGO_ENABLED=1 GO111MODULE=off go build -tags 'ABISupport systemd varlink seccomp'

podman-remote:
	CGO_ENABLED=1 GO111MODULE=off go build -tags '!ABISupport systemd seccomp' -o podmanV2-remote

clean:
	rm podmanV2 podmanV2-remote
