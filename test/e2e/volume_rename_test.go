//go:build linux || freebsd

package integration

import (
	. "github.com/containers/podman/v6/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman volume rename", func() {
	AfterEach(func() {
		podmanTest.CleanupVolume()
	})

	It("podman volume rename", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "rename", "myvol", "newvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("newvol"))

		check := podmanTest.Podman([]string{"volume", "inspect", "newvol"})
		check.WaitWithDefaultTimeout()
		Expect(check).Should(ExitCleanly())
		Expect(check.OutputToString()).To(ContainSubstring("newvol"))

		// Old name should no longer exist
		check = podmanTest.Podman([]string{"volume", "inspect", "myvol"})
		check.WaitWithDefaultTimeout()
		Expect(check).To(ExitWithError(125, "no such volume myvol"))
	})

	It("podman volume rename data persists", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Write data into the volume
		session = podmanTest.Podman([]string{"run", "--rm", "-v", "myvol:/data", ALPINE, "sh", "-c", "echo hello > /data/testfile"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "rename", "myvol", "newvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Verify data persists under new name
		session = podmanTest.Podman([]string{"run", "--rm", "-v", "newvol:/data", ALPINE, "cat", "/data/testfile"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("hello"))
	})

	It("podman volume rename fails when in use", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Create a container that uses the volume (but don't start it)
		session = podmanTest.Podman([]string{"create", "-v", "myvol:/data", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Rename should fail because a container references the volume
		session = podmanTest.Podman([]string{"volume", "rename", "myvol", "newvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "volume is being used"))
	})

	It("podman volume rename to existing name fails", func() {
		session := podmanTest.Podman([]string{"volume", "create", "vol1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "create", "vol2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "rename", "vol1", "vol2"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "volume already exists"))
	})

	It("podman volume rename nonexistent volume fails", func() {
		session := podmanTest.Podman([]string{"volume", "rename", "nosuchvol", "newvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "no such volume"))
	})

	It("podman volume rename to same name fails", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "rename", "myvol", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "new name is the same as the old name"))
	})

	It("podman volume rename with invalid name fails", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "rename", "myvol", "invalid/name"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "invalid argument"))
	})

	It("podman volume rename requires exactly 2 args", func() {
		session := podmanTest.Podman([]string{"volume", "rename", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "accepts 2 arg(s)"))
	})
})
