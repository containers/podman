package integration

var (
	STORAGE_OPTIONS          = "--storage-driver fuse-overlayfs"
	ROOTLESS_STORAGE_OPTIONS = "--storage-driver fuse-overlayfs"
	CACHE_IMAGES             = []string{ALPINE, BB, fedoraMinimal, nginx, redis, registry, infra, labels}
	nginx                    = "quay.io/libpod/alpine_nginx:latest"
	BB_GLIBC                 = "docker.io/library/busybox:glibc"
	registry                 = "docker.io/library/registry:2"
	labels                   = "quay.io/libpod/alpine_labels:latest"
)
