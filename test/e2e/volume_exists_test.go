package integration

import (
	"github.com/containers/storage/pkg/stringid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman volume exists", func() {

	AfterEach(func() {
		podmanTest.CleanupVolume()
	})

	It("podman volume exists", func() {
		vol := "vol" + stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"volume", "create", vol})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "exists", vol})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"volume", "exists", stringid.GenerateRandomID()})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
	})
})
