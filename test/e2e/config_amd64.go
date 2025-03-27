//go:build linux || freebsd

package integration

var (
	STORAGE_FS          = "overlay"                                                                                                                   //nolint:revive
	STORAGE_OPTIONS     = "--storage-driver overlay"                                                                                                  //nolint:revive
	ROOTLESS_STORAGE_FS = "overlay"                                                                                                                   //nolint:revive
	CACHE_IMAGES        = []string{ALPINE, BB, NGINX_IMAGE, REDIS_IMAGE, REGISTRY_IMAGE, INFRA_IMAGE, CITEST_IMAGE, HEALTHCHECK_IMAGE, SYSTEMD_IMAGE} //nolint:revive
	NGINX_IMAGE         = "quay.io/libpod/alpine_nginx:latest"                                                                                        //nolint:revive
	BB_GLIBC            = "docker.io/library/busybox:glibc"                                                                                           //nolint:revive
	REGISTRY_IMAGE      = "quay.io/libpod/registry:2.8.2"                                                                                             //nolint:revive
	CITEST_IMAGE        = "quay.io/libpod/testimage:20241011"                                                                                         //nolint:revive
	SYSTEMD_IMAGE       = "quay.io/libpod/systemd-image:20240124"                                                                                     //nolint:revive
	CIRROS_IMAGE        = "quay.io/libpod/cirros:latest"                                                                                              //nolint:revive
)
