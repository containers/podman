package integration

var (
	redis             = "quay.io/libpod/redis:alpine"
	fedoraMinimal     = "registry.fedoraproject.org/fedora-minimal:34"
	ALPINE            = "quay.io/libpod/alpine:latest"
	ALPINELISTTAG     = "quay.io/libpod/alpine:3.10.2"
	ALPINELISTDIGEST  = "quay.io/libpod/alpine@sha256:fa93b01658e3a5a1686dc3ae55f170d8de487006fb53a28efcd12ab0710a2e5f"
	ALPINEAMD64DIGEST = "quay.io/libpod/alpine@sha256:634a8f35b5f16dcf4aaa0822adc0b1964bb786fca12f6831de8ddc45e5986a00"
	ALPINEAMD64ID     = "961769676411f082461f9ef46626dd7a2d1e2b2a38e6a44364bcbecf51e66dd4"
	ALPINEARM64DIGEST = "quay.io/libpod/alpine@sha256:f270dcd11e64b85919c3bab66886e59d677cf657528ac0e4805d3c71e458e525"
	ALPINEARM64ID     = "915beeae46751fc564998c79e73a1026542e945ca4f73dc841d09ccc6c2c0672"
	infra             = "k8s.gcr.io/pause:3.2"
	BB                = "quay.io/libpod/busybox:latest"
	healthcheck       = "quay.io/libpod/alpine_healthcheck:latest"
	ImageCacheDir     = "/tmp/podman/imagecachedir"
	fedoraToolbox     = "registry.fedoraproject.org/fedora-toolbox:36"
	volumeTest        = "quay.io/libpod/volume-plugin-test-img:latest"

	// This image has seccomp profiles that blocks all syscalls.
	// The intention behind blocking all syscalls is to prevent
	// regressions in the future.  The required syscalls can vary
	// depending on which runtime we're using.
	alpineSeccomp = "quay.io/libpod/alpine-with-seccomp:label"
	// This image has a bogus/invalid seccomp profile which should
	// yield a json error when being read.
	alpineBogusSeccomp = "quay.io/libpod/alpine-with-bogus-seccomp:label"
)
