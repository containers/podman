package integration

import (
	"os"
	"os/user"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman top", func() {

	It("podman top without container name or id", func() {
		result := podmanTest.Podman([]string{"top"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))
	})

	It("podman top on bogus container", func() {
		result := podmanTest.Podman([]string{"top", "1234"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))
	})

	It("podman top on non-running container", func() {
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		result := podmanTest.Podman([]string{"top", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))
	})

	It("podman top on container", func() {
		session := podmanTest.Podman([]string{"run", "--name", "test", "-d", ALPINE, "top", "-d", "2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"top", "test"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(len(result.OutputToStringArray())).To(BeNumerically(">", 1))
	})

	It("podman container top on container", func() {
		session := podmanTest.Podman([]string{"container", "run", "--name", "test", "-d", ALPINE, "top", "-d", "2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"container", "top", "test"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(len(result.OutputToStringArray())).To(BeNumerically(">", 1))

		// Just a smoke test since groups may change over time.
		result = podmanTest.Podman([]string{"container", "top", "test", "groups", "hgroups"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(len(result.OutputToStringArray())).To(BeNumerically(">", 1))
	})

	It("podman top with options", func() {
		session := podmanTest.Podman([]string{"run", "-d", ALPINE, "top", "-d", "2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"top", session.OutputToString(), "pid", "%C", "args"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(len(result.OutputToStringArray())).To(BeNumerically(">", 1))

		result = podmanTest.Podman([]string{"container", "top", session.OutputToString(), "uid"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(len(result.OutputToStringArray())).To(BeNumerically(">", 1))
		Expect(result.OutputToStringArray()[1]).To(Equal("0"))

		user, err := user.Current()
		if err != nil {
			os.Exit(1)
		}

		result = podmanTest.Podman([]string{"container", "top", session.OutputToString(), "huid"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(len(result.OutputToStringArray())).To(BeNumerically(">", 1))
		Expect(result.OutputToStringArray()[1]).To(Equal(user.Uid))
	})

	It("podman top with ps(1) options", func() {
		session := podmanTest.Podman([]string{"run", "-d", fedoraMinimal, "sleep", "inf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"top", session.OutputToString(), "aux"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(len(result.OutputToStringArray())).To(BeNumerically(">", 1))

		result = podmanTest.Podman([]string{"top", session.OutputToString(), "ax -o args"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).To(Equal([]string{"COMMAND", "sleep inf"}))

		// Now make sure we use ps in the container with CAP_SYS_PTRACE
		session = podmanTest.Podman([]string{"run", "-d", "--cap-add=SYS_PTRACE", fedoraMinimal, "sleep", "inf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Because the image does not contain this must fail and we know we use the correct podman exec fallback.
		exec := podmanTest.Podman([]string{"top", session.OutputToString(), "aux"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(Exit(125))
		Expect(exec.ErrorToString()).Should(ContainSubstring("OCI runtime attempted to invoke a command that was not found"))
	})

	It("podman top with comma-separated options", func() {
		session := podmanTest.Podman([]string{"run", "-d", ALPINE, "top", "-d", "2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"top", session.OutputToString(), "user,pid,comm"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(len(result.OutputToStringArray())).To(BeNumerically(">", 1))
	})

	It("podman top on container invalid options", func() {
		top := podmanTest.RunTopContainer("")
		top.WaitWithDefaultTimeout()
		Expect(top).Should(ExitCleanly())
		cid := top.OutputToString()

		// We need to pass -eo to force executing ps in the Alpine container.
		// Alpines stripped down ps(1) is accepting any kind of weird input in
		// contrast to others, such that a `ps invalid` will silently ignore
		// the wrong input and still print the -ef output instead.
		result := podmanTest.Podman([]string{"top", cid, "-eo", "invalid"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(125))
	})

	It("podman top on privileged container", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"top", cid, "capeff"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).To(Equal([]string{"EFFECTIVE CAPS", "full"}))
	})
})
