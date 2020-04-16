all: podman podman-remote

BUILD_TAGS='ABISupport systemd varlink seccomp selinux exclude_graphdriver_devicemapper'

podman:
	CGO_ENABLED=1 GO111MODULE=off go build -tags $(BUILD_TAGS)

podman-remote:
	CGO_ENABLED=1 GO111MODULE=off go build -tags '!ABISupport systemd seccomp selinux' -o podmanV2-remote

clean:
	rm podmanV2 podmanV2-remote
