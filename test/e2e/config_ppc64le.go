package integration

var (
	STORAGE_FS               = "overlay"
	STORAGE_OPTIONS          = "--storage-driver overlay"
	ROOTLESS_STORAGE_FS      = "vfs"
	ROOTLESS_STORAGE_OPTIONS = "--storage-driver vfs"
	CACHE_IMAGES             = []string{ALPINE, BB, fedoraMinimal, NGINX_IMAGE, REDIS_IMAGE, INFRA_IMAGE, LABELS_IMAGE}
	NGINX_IMAGE              = "quay.io/libpod/alpine_nginx-ppc64le:latest"
	BB_GLIBC                 = "docker.io/ppc64le/busybox:glibc"
	LABELS_IMAGE             = "quay.io/libpod/alpine_labels-ppc64le:latest"
	REGISTRY_IMAGE           string
)
