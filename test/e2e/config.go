//go:build linux || freebsd

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2" //nolint:stylecheck  // FIXME
)

var (
	REDIS_IMAGE       = "quay.io/libpod/redis:alpine" //nolint:revive,stylecheck
	fedoraMinimal     = "quay.io/libpod/systemd-image:20240124"
	ALPINE            = "quay.io/libpod/alpine:latest"
	ALPINELISTTAG     = "quay.io/libpod/alpine:3.10.2"
	ALPINELISTDIGEST  = "quay.io/libpod/alpine@" + digestOrCachedTop("quay.io/libpod/alpine", "sha256:fa93b01658e3a5a1686dc3ae55f170d8de487006fb53a28efcd12ab0710a2e5f")
	ALPINEAMD64DIGEST = "quay.io/libpod/alpine@" + digestOrCachedArch("quay.io/libpod/alpine", "amd64", "sha256:634a8f35b5f16dcf4aaa0822adc0b1964bb786fca12f6831de8ddc45e5986a00")
	ALPINEAMD64ID     = ifCachedHardcoded("961769676411f082461f9ef46626dd7a2d1e2b2a38e6a44364bcbecf51e66dd4", "FIXME BROKEN")
	ALPINEARM64DIGEST = "quay.io/libpod/alpine@" + digestOrCachedArch("quay.io/libpod/alpine", "arm64", "sha256:f270dcd11e64b85919c3bab66886e59d677cf657528ac0e4805d3c71e458e525")
	ALPINEARM64ID     = "915beeae46751fc564998c79e73a1026542e945ca4f73dc841d09ccc6c2c0672"
	BUSYBOXARMDIGEST  = "quay.io/libpod/busybox@" + digestOrCachedArch("quay.io/libpod/busybox", "arm", "sha256:6655df04a3df853b029a5fac8836035ac4fab117800c9a6c4b69341bb5306c3d")
	INFRA_IMAGE       = "quay.io/libpod/k8s-pause:3.5" //nolint:revive,stylecheck
	BB                = "quay.io/libpod/busybox:latest"
	HEALTHCHECK_IMAGE = "quay.io/libpod/alpine_healthcheck:latest" //nolint:revive,stylecheck
	volumeTest        = "quay.io/libpod/volume-plugin-test-img:20220623"

	// ImageCacheDir is initialized at runtime.
	// e.g., filepath.Join(os.TempDir(), "imagecachedir")
	// This directory should be used by per-user.
	// Note: "ImageCacheDir" has nothing to do with "PODMAN_TEST_IMAGE_CACHE_DIR".
	ImageCacheDir = ""
)

func digestOrCachedTop(image string, standardDigest string) string {
	if !UsingCacheRegistry() {
		return standardDigest
	}
	cwd, _ := os.Getwd()
	cmd := exec.Command("skopeo", "inspect", "--raw", "docker://"+image, "--format", "{{.Digest}}")
	cmd.Env = append(os.Environ(), "CONTAINERS_REGISTRIES_CONF="+filepath.Join(cwd, "..", "registries-cached.conf"))
	out, err := cmd.Output()
	if err != nil {
		panic(fmt.Sprintf("Running %q: %s", cmd.String(), err.Error()))
	}
	GinkgoWriter.Printf("Digest of %q = %q", image, string(out))
	return strings.TrimSpace(string(out))
}

func digestOrCachedArch(image string, arch string, standardDigest string) string {
	if !UsingCacheRegistry() {
		return standardDigest
	}
	cwd, _ := os.Getwd()
	cmd := exec.Command("sh", "-c", `skopeo inspect --raw docker://`+image+
		` | jq '.manifests | map(select(.platform.architecture == "`+arch+`"))[0].digest'`)
	cmd.Env = append(os.Environ(), "CONTAINERS_REGISTRIES_CONF="+filepath.Join(cwd, "..", "registries-cached.conf"))
	out, err := cmd.Output()
	if err != nil {
		panic(fmt.Sprintf("Running %q: %s", cmd.String(), err.Error()))
	}
	GinkgoWriter.Printf("Digest of %q arc %s = %q", image, arch, string(out))
	return strings.TrimSpace(string(out))
}

func ifCachedHardcoded(standardDigest, cachedDigest string) string {
	if !UsingCacheRegistry() {
		return standardDigest
	}
	return cachedDigest
}
