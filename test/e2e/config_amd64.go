package integration

var (
	STORAGE_FS               = "vfs"                                                                                                                         //nolint:golint,stylecheck
	STORAGE_OPTIONS          = "--storage-driver vfs"                                                                                                        //nolint:golint,stylecheck
	ROOTLESS_STORAGE_FS      = "vfs"                                                                                                                         //nolint:golint,stylecheck
	ROOTLESS_STORAGE_OPTIONS = "--storage-driver vfs"                                                                                                        //nolint:golint,stylecheck
	CACHE_IMAGES             = []string{ALPINE, BB, fedoraMinimal, nginx, redis, registry, infra, labels, healthcheck, UBI_INIT, UBI_MINIMAL, fedoraToolbox} //nolint:golint,stylecheck
	nginx                    = "quay.io/libpod/alpine_nginx:latest"
	BB_GLIBC                 = "docker.io/library/busybox:glibc" //nolint:golint,stylecheck
	registry                 = "quay.io/libpod/registry:2.6"
	labels                   = "quay.io/libpod/alpine_labels:latest"
	UBI_MINIMAL              = "registry.access.redhat.com/ubi8-minimal" //nolint:golint,stylecheck
	UBI_INIT                 = "registry.access.redhat.com/ubi8-init"    //nolint:golint,stylecheck
	cirros                   = "quay.io/libpod/cirros:latest"
)
