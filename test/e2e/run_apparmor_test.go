//go:build !remote_testing && (linux || freebsd)

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.podman.io/common/pkg/apparmor"
	. "go.podman.io/podman/v6/test/utils"
)

// wip
func skipIfAppArmorEnabled() {
	if apparmor.IsEnabled() {
		Skip("Apparmor is enabled")
	}
}

func skipIfAppArmorDisabled() {
	if !apparmor.IsEnabled() {
		Skip("Apparmor is not enabled")
	}
}

var _ = Describe("Podman run", func() {
	It("podman run apparmor default", func() {
		skipIfAppArmorDisabled()
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		cid := session.OutputToString()
		// Verify that apparmor.Profile is being set
		inspect := podmanTest.InspectContainer(cid)
		Expect(inspect[0]).To(HaveField("AppArmorProfile", apparmor.Profile))
	})

	It("podman run no apparmor --privileged", func() {
		skipIfAppArmorDisabled()
		session := podmanTest.Podman([]string{"create", "--privileged", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		cid := session.OutputToString()
		// Verify that apparmor.Profile is being set
		inspect := podmanTest.InspectContainer(cid)
		Expect(inspect[0]).To(HaveField("AppArmorProfile", ""))
	})

	It("podman run no apparmor --security-opt=apparmor.Profile --privileged", func() {
		skipIfAppArmorDisabled()
		session := podmanTest.Podman([]string{"create", "--security-opt", fmt.Sprintf("apparmor=%s", apparmor.Profile), "--privileged", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		cid := session.OutputToString()
		// Verify that apparmor.Profile is being set
		inspect := podmanTest.InspectContainer(cid)
		Expect(inspect[0]).To(HaveField("AppArmorProfile", apparmor.Profile))
	})

	It("podman run apparmor aa-test-profile", func() {
		skipIfAppArmorDisabled()
		aaProfile := `
#include <tunables/global>
profile aa-test-profile flags=(attach_disconnected,mediate_deleted) {
  #include <abstractions/base>
  deny mount,
  deny /sys/[^f]*/** wklx,
  deny /sys/f[^s]*/** wklx,
  deny /sys/fs/[^c]*/** wklx,
  deny /sys/fs/c[^g]*/** wklx,
  deny /sys/fs/cg[^r]*/** wklx,
  deny /sys/firmware/efi/efivars/** rwklx,
  deny /sys/kernel/security/** rwklx,
}
`
		aaFile := filepath.Join(os.TempDir(), "aaFile")
		Expect(os.WriteFile(aaFile, []byte(aaProfile), 0o755)).To(Succeed())
		parse := SystemExec("apparmor_parser", []string{"-Kr", aaFile})
		Expect(parse).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"create", "--security-opt", "apparmor=aa-test-profile", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		cid := session.OutputToString()
		// Verify that apparmor.Profile is being set
		inspect := podmanTest.InspectContainer(cid)
		Expect(inspect[0]).To(HaveField("AppArmorProfile", "aa-test-profile"))
	})

	It("podman run apparmor invalid", func() {
		skipIfAppArmorDisabled()
		session := podmanTest.Podman([]string{"run", "--security-opt", "apparmor=invalid", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(126, `AppArmor profile "invalid" specified but not loaded`))
	})

	It("podman run apparmor unconfined", func() {
		skipIfAppArmorDisabled()
		session := podmanTest.Podman([]string{"create", "--security-opt", "apparmor=unconfined", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		cid := session.OutputToString()
		// Verify that apparmor.Profile is being set
		inspect := podmanTest.InspectContainer(cid)
		Expect(inspect[0]).To(HaveField("AppArmorProfile", "unconfined"))
	})

	It("podman run apparmor disabled --security-opt apparmor fails", func() {
		skipIfAppArmorEnabled()
		// Should fail if user specifies apparmor on disabled system
		session := podmanTest.Podman([]string{"create", "--security-opt", fmt.Sprintf("apparmor=%s", apparmor.Profile), ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, fmt.Sprintf(`apparmor profile "%s" specified, but Apparmor is not enabled on this system`, apparmor.Profile)))
	})

	It("podman run apparmor disabled no default", func() {
		skipIfAppArmorEnabled()
		// Should succeed if user specifies apparmor on disabled system
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		cid := session.OutputToString()
		// Verify that apparmor.Profile is being set
		inspect := podmanTest.InspectContainer(cid)
		Expect(inspect[0]).To(HaveField("AppArmorProfile", ""))
	})

	It("podman run apparmor disabled unconfined", func() {
		skipIfAppArmorEnabled()

		session := podmanTest.Podman([]string{"create", "--security-opt", "apparmor=unconfined", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		cid := session.OutputToString()
		// Verify that apparmor.Profile is being set
		inspect := podmanTest.InspectContainer(cid)
		Expect(inspect[0]).To(HaveField("AppArmorProfile", ""))
	})

	It("podman run with uidmap and restart policy should restart successfully", func() {
		skipIfAppArmorDisabled()
		SkipIfRootless("Test requires root")

		// Create a container with UID/GID mappings and restart policy
		session := podmanTest.Podman([]string{
			"run", "-d", "--name", "test-uidmap-restart",
			"--restart", "always",
			"--uidmap", "0:200000:65536",
			"--gidmap", "0:200000:65536",
			ALPINE, "sleep", "4",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Wait for the container to exit
		session = podmanTest.Podman([]string{"wait", "test-uidmap-restart"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Verify the container restarted (restart count should be > 0)
		Eventually(func() int {
			inspect := podmanTest.InspectContainer("test-uidmap-restart")
			return int(inspect[0].RestartCount)
		}, 10*time.Second, 500*time.Millisecond).Should(BeNumerically(">", 0))

		// Verify it's running after restart
		inspect := podmanTest.InspectContainer("test-uidmap-restart")
		Expect(inspect[0]).To(HaveField("State", "running"))

		// Clean up
		session = podmanTest.Podman([]string{"rm", "-f", "test-uidmap-restart"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})
})
