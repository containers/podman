//go:build linux || freebsd

package integration

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman buildx inspect", func() {
	It("podman buildx inspect", func() {
		SkipIfRemote("binfmt-misc emulation only probed in local mode")

		session := podmanTest.PodmanExitCleanly("buildx", "inspect")
		out := session.OutputToString()

		session_bootstrap := podmanTest.PodmanExitCleanly("buildx", "inspect", "--bootstrap")
		out_bootstrap := session_bootstrap.OutputToString()
		Expect(out_bootstrap).To(Equal(out), "Output of 'podman buildx inspect' and 'podman buildx inspect --bootstrap' should be the same")

		emuInfo := podmanTest.PodmanExitCleanly("info", "--format", "{{json .Host.EmulatedArchitectures}}")
		var emuArchs []string
		Expect(json.Unmarshal([]byte(emuInfo.OutputToString()), &emuArchs)).To(Succeed())

		nativeInfo := podmanTest.PodmanExitCleanly("info", "--format", "{{.Host.OS}}/{{.Host.Arch}}")
		nativePlat := strings.TrimSpace(nativeInfo.OutputToString())
		Expect(nativePlat).ToNot(BeEmpty())

		expected := append(emuArchs, nativePlat)

		for _, p := range expected {
			re := regexp.MustCompile(`(?s)Platforms:.*\b` + regexp.QuoteMeta(p) + `\b`)
			Expect(out).To(MatchRegexp(re.String()), "missing %q in:\n%s", p, out)
		}
	})

	It("podman-remote buildx inspect", func() {
		session := podmanTest.PodmanExitCleanly("buildx", "inspect")
		out := session.OutputToString()

		session_bootstrap := podmanTest.PodmanExitCleanly("buildx", "inspect", "--bootstrap")
		out_bootstrap := session_bootstrap.OutputToString()
		Expect(out_bootstrap).To(Equal(out), "Output of 'podman-remote buildx inspect' and 'podman-remote buildx inspect --bootstrap' should be the same")

		nativeInfo := podmanTest.PodmanExitCleanly("info", "--format", "{{.Host.OS}}/{{.Host.Arch}}")
		fmt.Fprintf(GinkgoWriter, "nativeInfo: %s\n", nativeInfo.OutputToString())
		nativePlatform := strings.TrimSpace(nativeInfo.OutputToString())

		Expect(nativePlatform).ToNot(BeEmpty())

		re := regexp.MustCompile(`(?s)Platforms:.*\b` + regexp.QuoteMeta(nativePlatform) + `\b`)
		Expect(out).To(MatchRegexp(re.String()), "missing native %q in:\n%s", nativePlatform, out)
	})
})
