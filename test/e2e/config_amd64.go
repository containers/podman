//go:build linux || freebsd

package integration

var (
	STORAGE_FS          = "overlay"                                                                                                                   //nolint:revive,stylecheck
	STORAGE_OPTIONS     = "--storage-driver overlay"                                                                                                  //nolint:revive,stylecheck
	ROOTLESS_STORAGE_FS = "overlay"                                                                                                                   //nolint:revive,stylecheck
	CACHE_IMAGES        = []string{ALPINE, BB, NGINX_IMAGE, REDIS_IMAGE, REGISTRY_IMAGE, INFRA_IMAGE, CITEST_IMAGE, HEALTHCHECK_IMAGE, SYSTEMD_IMAGE} //nolint:revive,stylecheck
	NGINX_IMAGE         = "quay.io/libpod/alpine_nginx:latest"                                                                                        //nolint:revive,stylecheck
	BB_GLIBC            = "docker.io/library/busybox:glibc"                                                                                           //nolint:revive,stylecheck
	REGISTRY_IMAGE      = "quay.io/libpod/registry:2.8.2"                                                                                             //nolint:revive,stylecheck
	CITEST_IMAGE        = "quay.io/libpod/testimage:20241011"                                                                                         //nolint:revive,stylecheck
	SYSTEMD_IMAGE       = "quay.io/libpod/systemd-image:20240124"                                                                                     //nolint:revive,stylecheck
	CIRROS_IMAGE        = "quay.io/libpod/cirros:latest"                                                                                              //nolint:revive,stylecheck
)
