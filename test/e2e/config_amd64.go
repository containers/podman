package integration

var (
	STORAGE_FS               = "vfs"                                                                                                                                                             //nolint:revive,stylecheck
	STORAGE_OPTIONS          = "--storage-driver vfs"                                                                                                                                            //nolint:revive,stylecheck
	ROOTLESS_STORAGE_FS      = "vfs"                                                                                                                                                             //nolint:revive,stylecheck
	ROOTLESS_STORAGE_OPTIONS = "--storage-driver vfs"                                                                                                                                            //nolint:revive,stylecheck
	CACHE_IMAGES             = []string{ALPINE, BB, fedoraMinimal, NGINX_IMAGE, REDIS_IMAGE, REGISTRY_IMAGE, INFRA_IMAGE, LABELS_IMAGE, HEALTHCHECK_IMAGE, UBI_INIT, UBI_MINIMAL, fedoraToolbox} //nolint:revive,stylecheck
	NGINX_IMAGE              = "quay.io/libpod/alpine_nginx:latest"                                                                                                                              //nolint:revive,stylecheck
	BB_GLIBC                 = "docker.io/library/busybox:glibc"                                                                                                                                 //nolint:revive,stylecheck
	REGISTRY_IMAGE           = "quay.io/libpod/registry:2.8"                                                                                                                                     //nolint:revive,stylecheck
	LABELS_IMAGE             = "quay.io/libpod/alpine_labels:latest"                                                                                                                             //nolint:revive,stylecheck
	UBI_MINIMAL              = "registry.access.redhat.com/ubi8-minimal"                                                                                                                         //nolint:revive,stylecheck
	UBI_INIT                 = "registry.access.redhat.com/ubi8-init"                                                                                                                            //nolint:revive,stylecheck
	CIRROS_IMAGE             = "quay.io/libpod/cirros:latest"                                                                                                                                    //nolint:revive,stylecheck
)
