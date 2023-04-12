package integration

import (
	"fmt"
	"os"
	"strings"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman run networking", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
		SkipIfCNI(podmanTest)
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("Aardvark Test 1: One container", func() {
		netName := createNetworkName("Test")
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName)
		Expect(session).Should(Exit(0))

		ctrID := podmanTest.Podman([]string{"run", "-dt", "--name", "aone", "--network", netName, NGINX_IMAGE})
		ctrID.WaitWithDefaultTimeout()
		Expect(ctrID).Should(Exit(0))
		cid := ctrID.OutputToString()

		ctrIP := podmanTest.Podman([]string{"inspect", "--format", fmt.Sprintf(`{{.NetworkSettings.Networks.%s.IPAddress}}`, netName), cid})
		ctrIP.WaitWithDefaultTimeout()
		Expect(ctrIP).Should(Exit(0))
		cip := ctrIP.OutputToString()
		Expect(cip).To(MatchRegexp(IPRegex))

		digShort(cid, "aone", cip, podmanTest)

		reverseLookup := podmanTest.Podman([]string{"exec", cid, "dig", "+short", "-x", cip})
		reverseLookup.WaitWithDefaultTimeout()
		Expect(reverseLookup).Should(Exit(0))
		revListArray := reverseLookup.OutputToStringArray()
		Expect(revListArray).Should(HaveLen(2))
		Expect(strings.TrimRight(revListArray[0], ".")).To(Equal("aone"))
		Expect(strings.TrimRight(revListArray[1], ".")).To(Equal(cid[:12]))

	})

	It("Aardvark Test 2: Two containers, same subnet", func() {
		netName := createNetworkName("Test")
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName)
		Expect(session).Should(Exit(0))

		ctr1 := podmanTest.Podman([]string{"run", "-dt", "--name", "aone", "--network", netName, NGINX_IMAGE})
		ctr1.WaitWithDefaultTimeout()
		Expect(ctr1).Should(Exit(0))
		cid1 := ctr1.OutputToString()

		ctrIP1 := podmanTest.Podman([]string{"inspect", "--format", fmt.Sprintf(`{{.NetworkSettings.Networks.%s.IPAddress}}`, netName), cid1})
		ctrIP1.WaitWithDefaultTimeout()
		Expect(ctrIP1).Should(Exit(0))
		cip1 := ctrIP1.OutputToString()
		Expect(cip1).To(MatchRegexp(IPRegex))

		ctr2 := podmanTest.Podman([]string{"run", "-dt", "--name", "atwo", "--network", netName, NGINX_IMAGE})
		ctr2.WaitWithDefaultTimeout()
		Expect(ctr2).Should(Exit(0))
		cid2 := ctr2.OutputToString()

		ctrIP2 := podmanTest.Podman([]string{"inspect", "--format", fmt.Sprintf(`{{.NetworkSettings.Networks.%s.IPAddress}}`, netName), cid2})
		ctrIP2.WaitWithDefaultTimeout()
		Expect(ctrIP2).Should(Exit(0))
		cip2 := ctrIP2.OutputToString()
		Expect(cip2).To(MatchRegexp(IPRegex))

		digShort("aone", "atwo", cip2, podmanTest)

		digShort("atwo", "aone", cip1, podmanTest)

		reverseLookup12 := podmanTest.Podman([]string{"exec", cid1, "dig", "+short", "-x", cip2})
		reverseLookup12.WaitWithDefaultTimeout()
		Expect(reverseLookup12).Should(Exit(0))
		revListArray12 := reverseLookup12.OutputToStringArray()
		Expect(revListArray12).Should(HaveLen(2))
		Expect(strings.TrimRight(revListArray12[0], ".")).To(Equal("atwo"))
		Expect(strings.TrimRight(revListArray12[1], ".")).To(Equal(cid2[:12]))

		reverseLookup21 := podmanTest.Podman([]string{"exec", cid2, "dig", "+short", "-x", cip1})
		reverseLookup21.WaitWithDefaultTimeout()
		Expect(reverseLookup21).Should(Exit(0))
		revListArray21 := reverseLookup21.OutputToStringArray()
		Expect(revListArray21).Should(HaveLen(2))
		Expect(strings.TrimRight(revListArray21[0], ".")).To(Equal("aone"))
		Expect(strings.TrimRight(revListArray21[1], ".")).To(Equal(cid1[:12]))

	})

	It("Aardvark Test 3: Two containers, same subnet w/aliases", func() {
		netName := createNetworkName("Test")
		session := podmanTest.Podman([]string{"network", "create", netName})
		session.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netName)
		Expect(session).Should(Exit(0))

		ctr1 := podmanTest.Podman([]string{"run", "-dt", "--name", "aone", "--network", netName, "--network-alias", "alias_a1,alias_1a", NGINX_IMAGE})
		ctr1.WaitWithDefaultTimeout()
		Expect(ctr1).Should(Exit(0))

		ctrIP1 := podmanTest.Podman([]string{"inspect", "--format", fmt.Sprintf(`{{.NetworkSettings.Networks.%s.IPAddress}}`, netName), "aone"})
		ctrIP1.WaitWithDefaultTimeout()
		Expect(ctrIP1).Should(Exit(0))
		cip1 := ctrIP1.OutputToString()
		Expect(cip1).To(MatchRegexp(IPRegex))

		ctr2 := podmanTest.Podman([]string{"run", "-dt", "--name", "atwo", "--network", netName, "--network-alias", "alias_a2,alias_2a", NGINX_IMAGE})
		ctr2.WaitWithDefaultTimeout()
		Expect(ctr2).Should(Exit(0))

		ctrIP2 := podmanTest.Podman([]string{"inspect", "--format", fmt.Sprintf(`{{.NetworkSettings.Networks.%s.IPAddress}}`, netName), "atwo"})
		ctrIP2.WaitWithDefaultTimeout()
		Expect(ctrIP2).Should(Exit(0))
		cip2 := ctrIP2.OutputToString()
		Expect(cip2).To(MatchRegexp(IPRegex))

		digShort("aone", "atwo", cip2, podmanTest)

		digShort("aone", "alias_a2", cip2, podmanTest)

		digShort("aone", "alias_2a", cip2, podmanTest)

		digShort("atwo", "aone", cip1, podmanTest)

		digShort("atwo", "alias_a1", cip1, podmanTest)

		digShort("atwo", "alias_1a", cip1, podmanTest)

	})

	It("Aardvark Test 4: Two containers, different subnets", func() {
		netNameA := createNetworkName("TestA")
		sessionA := podmanTest.Podman([]string{"network", "create", netNameA})
		sessionA.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netNameA)
		Expect(sessionA).Should(Exit(0))

		netNameB := createNetworkName("TestB")
		sessionB := podmanTest.Podman([]string{"network", "create", netNameB})
		sessionB.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netNameB)
		Expect(sessionB).Should(Exit(0))

		ctrA1 := podmanTest.Podman([]string{"run", "-dt", "--name", "aone", "--network", netNameA, NGINX_IMAGE})
		ctrA1.WaitWithDefaultTimeout()
		cidA1 := ctrA1.OutputToString()

		ctrB1 := podmanTest.Podman([]string{"run", "-dt", "--name", "bone", "--network", netNameB, NGINX_IMAGE})
		ctrB1.WaitWithDefaultTimeout()
		cidB1 := ctrB1.OutputToString()

		ctrIPA1 := podmanTest.Podman([]string{"inspect", "--format", fmt.Sprintf(`{{.NetworkSettings.Networks.%s.IPAddress}}`, netNameA), cidA1})
		ctrIPA1.WaitWithDefaultTimeout()
		Expect(ctrIPA1).Should(Exit(0))
		cipA1 := ctrIPA1.OutputToString()
		Expect(cipA1).To(MatchRegexp(IPRegex))

		ctrIPB1 := podmanTest.Podman([]string{"inspect", "--format", fmt.Sprintf(`{{.NetworkSettings.Networks.%s.IPAddress}}`, netNameB), cidB1})
		ctrIPB1.WaitWithDefaultTimeout()
		Expect(ctrIPB1).Should(Exit(0))
		cipB1 := ctrIPB1.OutputToString()
		Expect(cipB1).To(MatchRegexp(IPRegex))

		resA1B1 := podmanTest.Podman([]string{"exec", "aone", "dig", "+short", "bone"})
		resA1B1.WaitWithDefaultTimeout()
		Expect(resA1B1).Should(Exit(0))
		Expect(resA1B1.OutputToString()).To(Equal(""))

		resB1A1 := podmanTest.Podman([]string{"exec", "bone", "dig", "+short", "aone"})
		resB1A1.WaitWithDefaultTimeout()
		Expect(resB1A1).Should(Exit(0))
		Expect(resB1A1.OutputToString()).To(Equal(""))
	})

	It("Aardvark Test 5: Two containers on their own subnets, one container on both", func() {
		netNameA := createNetworkName("TestA")
		sessionA := podmanTest.Podman([]string{"network", "create", netNameA})
		sessionA.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netNameA)
		Expect(sessionA).Should(Exit(0))

		netNameB := createNetworkName("TestB")
		sessionB := podmanTest.Podman([]string{"network", "create", netNameB})
		sessionB.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netNameB)
		Expect(sessionB).Should(Exit(0))

		ctrA1 := podmanTest.Podman([]string{"run", "-dt", "--name", "aone", "--network", netNameA, NGINX_IMAGE})
		ctrA1.WaitWithDefaultTimeout()
		cidA1 := ctrA1.OutputToString()

		ctrIPA1 := podmanTest.Podman([]string{"inspect", "--format", fmt.Sprintf(`{{.NetworkSettings.Networks.%s.IPAddress}}`, netNameA), cidA1})
		ctrIPA1.WaitWithDefaultTimeout()
		Expect(ctrIPA1).Should(Exit(0))
		cipA1 := ctrIPA1.OutputToString()
		Expect(cipA1).To(MatchRegexp(IPRegex))

		ctrB1 := podmanTest.Podman([]string{"run", "-dt", "--name", "bone", "--network", netNameB, NGINX_IMAGE})
		ctrB1.WaitWithDefaultTimeout()
		cidB1 := ctrB1.OutputToString()

		ctrIPB1 := podmanTest.Podman([]string{"inspect", "--format", fmt.Sprintf(`{{.NetworkSettings.Networks.%s.IPAddress}}`, netNameB), cidB1})
		ctrIPB1.WaitWithDefaultTimeout()
		Expect(ctrIPB1).Should(Exit(0))
		cipB1 := ctrIPB1.OutputToString()
		Expect(cipB1).To(MatchRegexp(IPRegex))

		ctrA2B2 := podmanTest.Podman([]string{"run", "-dt", "--name", "atwobtwo", "--network", netNameA, "--network", netNameB, NGINX_IMAGE})
		ctrA2B2.WaitWithDefaultTimeout()
		cidA2B2 := ctrA2B2.OutputToString()

		ctrIPA2B21 := podmanTest.Podman([]string{"inspect", "--format", fmt.Sprintf(`{{.NetworkSettings.Networks.%s.IPAddress}}`, netNameA), cidA2B2})
		ctrIPA2B21.WaitWithDefaultTimeout()
		Expect(ctrIPA2B21).Should(Exit(0))
		cipA2B21 := ctrIPA2B21.OutputToString()
		Expect(cipA2B21).To(MatchRegexp(IPRegex))

		ctrIPA2B22 := podmanTest.Podman([]string{"inspect", "--format", fmt.Sprintf(`{{.NetworkSettings.Networks.%s.IPAddress}}`, netNameB), cidA2B2})
		ctrIPA2B22.WaitWithDefaultTimeout()
		Expect(ctrIPA2B22).Should(Exit(0))
		cipA2B22 := ctrIPA2B22.OutputToString()
		Expect(cipA2B22).To(MatchRegexp(IPRegex))

		digShort("aone", "atwobtwo", cipA2B21, podmanTest)

		digShort("bone", "atwobtwo", cipA2B22, podmanTest)

		digShort("atwobtwo", "aone", cipA1, podmanTest)

		digShort("atwobtwo", "bone", cipB1, podmanTest)
	})

	It("Aardvark Test 6: Three subnets, first container on 1/2 and second on 2/3, w/ network aliases", func() {
		netNameA := createNetworkName("TestA")
		sessionA := podmanTest.Podman([]string{"network", "create", netNameA})
		sessionA.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netNameA)
		Expect(sessionA).Should(Exit(0))

		netNameB := createNetworkName("TestB")
		sessionB := podmanTest.Podman([]string{"network", "create", netNameB})
		sessionB.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netNameB)
		Expect(sessionB).Should(Exit(0))

		netNameC := createNetworkName("TestC")
		sessionC := podmanTest.Podman([]string{"network", "create", netNameC})
		sessionC.WaitWithDefaultTimeout()
		defer podmanTest.removeNetwork(netNameC)
		Expect(sessionC).Should(Exit(0))

		ctrA := podmanTest.Podman([]string{"run", "-dt", "--name", "aone", "--network", netNameA, NGINX_IMAGE})
		ctrA.WaitWithDefaultTimeout()
		Expect(ctrA).Should(Exit(0))

		ctrC := podmanTest.Podman([]string{"run", "-dt", "--name", "cone", "--network", netNameC, NGINX_IMAGE})
		ctrC.WaitWithDefaultTimeout()
		Expect(ctrC).Should(Exit(0))

		ctrnetAB1 := podmanTest.Podman([]string{"network", "connect", "--alias", "testB1_nw", netNameB, "aone"})
		ctrnetAB1.WaitWithDefaultTimeout()
		Expect(ctrnetAB1).Should(Exit(0))

		ctrIPAB1 := podmanTest.Podman([]string{"inspect", "--format", fmt.Sprintf(`{{.NetworkSettings.Networks.%s.IPAddress}}`, netNameB), "aone"})
		ctrIPAB1.WaitWithDefaultTimeout()
		Expect(ctrIPAB1).Should(Exit(0))
		cipAB1 := ctrIPAB1.OutputToString()

		ctrnetCB2 := podmanTest.Podman([]string{"network", "connect", "--alias", "testB2_nw", netNameB, "cone"})
		ctrnetCB2.WaitWithDefaultTimeout()
		Expect(ctrnetCB2).Should(Exit(0))

		ctrIPCB2 := podmanTest.Podman([]string{"inspect", "--format", fmt.Sprintf(`{{.NetworkSettings.Networks.%s.IPAddress}}`, netNameB), "cone"})
		ctrIPCB2.WaitWithDefaultTimeout()
		Expect(ctrIPCB2).Should(Exit(0))
		cipCB2 := ctrIPCB2.OutputToString()

		digShort("aone", "testB2_nw", cipCB2, podmanTest)

		digShort("cone", "testB1_nw", cipAB1, podmanTest)
	})

})
