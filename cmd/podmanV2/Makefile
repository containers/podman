all: podman podman-remote

podman:
	CGO_ENABLED=1 GO111MODULE=off go build -tags 'ABISupport systemd varlink seccomp selinux'

podman-remote:
	CGO_ENABLED=1 GO111MODULE=off go build -tags '!ABISupport systemd seccomp selinux' -o podmanV2-remote

clean:
	rm podmanV2 podmanV2-remote
