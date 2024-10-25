package integration

import (
	"fmt"

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
			To(Equal(map[string][]define.InspectHostPort{"8787/udp": nil, "99/sctp": nil}))
		Expect(data[0].Config.ExposedPorts).
			To(Equal(map[string]struct{}{"8787/udp": {}, "99/sctp": {}}))

		session = podmanTest.Podman([]string{"ps", "--format", "{{.Ports}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("99/sctp, 8787/udp"))
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

	It("podman inspect exposed ports includes published ports", func() {
		c1 := "ctr1"
		c1s := podmanTest.Podman([]string{"run", "-d", "--expose", "22/tcp", "-p", "8080:80/tcp", "--name", c1, ALPINE, "top"})
		c1s.WaitWithDefaultTimeout()
		Expect(c1s).Should(ExitCleanly())

		c2 := "ctr2"
		c2s := podmanTest.Podman([]string{"run", "-d", "--net", fmt.Sprintf("container:%s", c1), "--name", c2, ALPINE, "top"})
		c2s.WaitWithDefaultTimeout()
		Expect(c2s).Should(ExitCleanly())

		data1 := podmanTest.InspectContainer(c1)
		Expect(data1).To(HaveLen(1))
		Expect(data1[0].Config.ExposedPorts).
			To(Equal(map[string]struct{}{"22/tcp": {}, "80/tcp": {}}))

		data2 := podmanTest.InspectContainer(c2)
		Expect(data2).To(HaveLen(1))
		Expect(data2[0].Config.ExposedPorts).To(BeNil())
	})
})
