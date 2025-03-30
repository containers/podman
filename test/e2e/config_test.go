//go:build linux || freebsd

package integration

var (
	fedoraMinimal = "quay.io/libpod/systemd-image:20240124"
	volumeTest    = "quay.io/libpod/volume-plugin-test-img:20220623"
)
