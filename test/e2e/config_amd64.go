package integration

var (
	STORAGE_FS               = "vfs"                                                                                                                         //nolint:revive,stylecheck
	STORAGE_OPTIONS          = "--storage-driver vfs"                                                                                                        //nolint:revive,stylecheck
	ROOTLESS_STORAGE_FS      = "vfs"                                                                                                                         //nolint:revive,stylecheck
	ROOTLESS_STORAGE_OPTIONS = "--storage-driver vfs"                                                                                                        //nolint:revive,stylecheck
	CACHE_IMAGES             = []string{ALPINE, BB, fedoraMinimal, nginx, redis, registry, infra, labels, healthcheck, UBI_INIT, UBI_MINIMAL, fedoraToolbox} //nolint:revive,stylecheck
	nginx                    = "quay.io/libpod/alpine_nginx:latest"
	BB_GLIBC                 = "docker.io/library/busybox:glibc" //nolint:revive,stylecheck
	registry                 = "quay.io/libpod/registry:2.6"
	labels                   = "quay.io/libpod/alpine_labels:latest"
	UBI_MINIMAL              = "registry.access.redhat.com/ubi8-minimal" //nolint:revive,stylecheck
	UBI_INIT                 = "registry.access.redhat.com/ubi8-init"    //nolint:revive,stylecheck
	cirros                   = "quay.io/libpod/cirros:latest"
)
