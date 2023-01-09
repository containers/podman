package integration

var (
	STORAGE_FS               = "vfs"                                                                                                                                                     //nolint:revive,stylecheck
	STORAGE_OPTIONS          = "--storage-driver vfs"                                                                                                                                    //nolint:revive,stylecheck
	ROOTLESS_STORAGE_FS      = "vfs"                                                                                                                                                     //nolint:revive,stylecheck
	ROOTLESS_STORAGE_OPTIONS = "--storage-driver vfs"                                                                                                                                    //nolint:revive,stylecheck
	CACHE_IMAGES             = []string{ALPINE, BB, fedoraMinimal, NGINX_IMAGE, REDIS_IMAGE, REGISTRY_IMAGE, INFRA_IMAGE, LABELS_IMAGE, HEALTHCHECK_IMAGE, SYSTEMD_IMAGE, fedoraToolbox} //nolint:revive,stylecheck
	NGINX_IMAGE              = "quay.io/lsm5/alpine_nginx-aarch64:latest"                                                                                                                //nolint:revive,stylecheck
	BB_GLIBC                 = "docker.io/library/busybox:glibc"                                                                                                                         //nolint:revive,stylecheck
	REGISTRY_IMAGE           = "quay.io/libpod/registry:2.8"                                                                                                                             //nolint:revive,stylecheck
	LABELS_IMAGE             = "quay.io/libpod/alpine_labels:latest"                                                                                                                     //nolint:revive,stylecheck
	SYSTEMD_IMAGE            = "quay.io/libpod/systemd-image:20230106"                                                                                                                   //nolint:revive,stylecheck
	CIRROS_IMAGE             = "quay.io/libpod/cirros:latest"                                                                                                                            //nolint:revive,stylecheck
)
