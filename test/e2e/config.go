package integration

var (
	redis             = "docker.io/library/redis:alpine"
	fedoraMinimal     = "quay.io/libpod/fedora-minimal:latest"
	ALPINE            = "docker.io/library/alpine:latest"
	ALPINELISTTAG     = "docker.io/library/alpine:3.10.2"
	ALPINELISTDIGEST  = "docker.io/library/alpine@sha256:72c42ed48c3a2db31b7dafe17d275b634664a708d901ec9fd57b1529280f01fb"
	ALPINEAMD64DIGEST = "docker.io/library/alpine@sha256:acd3ca9941a85e8ed16515bfc5328e4e2f8c128caa72959a58a127b7801ee01f"
	ALPINEAMD64ID     = "961769676411f082461f9ef46626dd7a2d1e2b2a38e6a44364bcbecf51e66dd4"
	ALPINEARM64DIGEST = "docker.io/library/alpine@sha256:db7f3dcef3d586f7dd123f107c93d7911515a5991c4b9e51fa2a43e46335a43e"
	ALPINEARM64ID     = "915beeae46751fc564998c79e73a1026542e945ca4f73dc841d09ccc6c2c0672"
	infra             = "k8s.gcr.io/pause:3.2"
	BB                = "docker.io/library/busybox:latest"
	healthcheck       = "docker.io/libpod/alpine_healthcheck:latest"
	ImageCacheDir     = "/tmp/podman/imagecachedir"

	// This image has seccomp profiles that blocks all syscalls.
	// The intention behind blocking all syscalls is to prevent
	// regressions in the future.  The required syscalls can vary
	// depending on which runtime we're using.
	alpineSeccomp = "docker.io/libpod/alpine-with-seccomp:label"
	// This image has a bogus/invalid seccomp profile which should
	// yield a json error when being read.
	alpineBogusSeccomp = "docker.io/libpod/alpine-with-bogus-seccomp:label"

	// v2fail is a temporary variable to help us track
	// tests that fail in v2
	v2fail = "does not pass integration tests with v2 podman"

	// v2remotefail is a temporary variable to help us track
	// tests that fail in v2 remote
	v2remotefail = "does not pass integration tests with v2 podman remote"
)
