package integration

import (
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/annotations"
	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman container inspect", func() {

	It("podman inspect a container for the container manager annotation", func() {
		const testContainer = "container-inspect-test-1"
		setup := podmanTest.RunTopContainer(testContainer)
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		data := podmanTest.InspectContainer(testContainer)
		Expect(data[0].Config.Annotations[annotations.ContainerManager]).
			To(Equal(annotations.ContainerManagerLibpod))
	})

	It("podman inspect shows exposed ports", func() {
		name := "testcon"
		session := podmanTest.Podman([]string{"run", "-d", "--stop-timeout", "0", "--expose", "8787/udp", "--name", name, ALPINE, "sleep", "inf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		data := podmanTest.InspectContainer(name)

		Expect(data).To(HaveLen(1))
		Expect(data[0].NetworkSettings.Ports).
			To(Equal(map[string][]define.InspectHostPort{"8787/udp": nil}))
	})

	It("podman inspect shows exposed ports on image", func() {
		name := "testcon"
		session := podmanTest.Podman([]string{"run", "-d", "--expose", "8989", "--name", name, NGINX_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		data := podmanTest.InspectContainer(name)
		Expect(data).To(HaveLen(1))
		Expect(data[0].NetworkSettings.Ports).
			To(Equal(map[string][]define.InspectHostPort{"80/tcp": nil, "8989/tcp": nil}))
	})
})
