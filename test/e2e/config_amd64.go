package integration

var (
	STORAGE_OPTIONS          = "--storage-driver vfs"
	ROOTLESS_STORAGE_OPTIONS = "--storage-driver vfs"
	CACHE_IMAGES             = []string{ALPINE, BB, fedoraMinimal, nginx, redis, registry, infra, labels}
	nginx                    = "quay.io/baude/alpine_nginx:latest"
	BB_GLIBC                 = "docker.io/library/busybox:glibc"
	registry                 = "docker.io/library/registry:2"
	labels                   = "quay.io/baude/alpine_labels:latest"
)
