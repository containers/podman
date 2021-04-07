package integration

var (
	STORAGE_FS               = "vfs"
	STORAGE_OPTIONS          = "--storage-driver vfs"
	ROOTLESS_STORAGE_FS      = "vfs"
	ROOTLESS_STORAGE_OPTIONS = "--storage-driver vfs"
	CACHE_IMAGES             = []string{ALPINE, BB, fedoraMinimal, nginx, redis, registry, infra, labels, healthcheck, ubi_init, ubi_minimal, fedoraToolbox}
	nginx                    = "quay.io/libpod/alpine_nginx:latest"
	BB_GLIBC                 = "docker.io/library/busybox:glibc"
	registry                 = "quay.io/libpod/registry:2.6"
	labels                   = "quay.io/libpod/alpine_labels:latest"
	ubi_minimal              = "registry.access.redhat.com/ubi8-minimal"
	ubi_init                 = "registry.access.redhat.com/ubi8-init"
	cirros                   = "quay.io/libpod/cirros:latest"
)
