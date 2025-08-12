//go:build linux || freebsd

package integration

import (
	"fmt"
	"time"

	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman volume ls", func() {

	AfterEach(func() {
		podmanTest.CleanupVolume()
	})

	It("podman ls volume", func() {
		// https://github.com/containers/podman/issues/25911
		// Output header for volume ls even when empty
		empty := podmanTest.PodmanExitCleanly("volume", "ls")
		Expect(empty.OutputToString()).To(ContainSubstring("DRIVER"))
		Expect(empty.OutputToString()).To(ContainSubstring("VOLUME NAME"))
		Expect(empty.ErrorToStringArray()).To(HaveLen(1))

		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(2))
	})

	It("podman ls volume filter with comma label", func() {
		session := podmanTest.Podman([]string{"volume", "create", "--label", "test=with,comma", "myvol3"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "label=test=with,comma"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(2))
	})

	It("podman ls volume filter with a key pattern", func() {
		session := podmanTest.Podman([]string{"volume", "create", "--label", "helloworld=world", "myvol2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "label=hello*"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(2))
	})

	It("podman ls volume with JSON format", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls", "--format", "json"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(BeValidJSON())
	})

	It("podman ls volume with Go template", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls", "--format", "table {{.Name}} {{.Driver}} {{.Scope}}"})
		session.WaitWithDefaultTimeout()

		Expect(session).Should(ExitCleanly())
		arr := session.OutputToStringArray()
		Expect(arr).To(HaveLen(2))
		Expect(arr[0]).To(ContainSubstring("NAME"))
		Expect(arr[1]).To(ContainSubstring("myvol"))
	})

	It("podman ls volume with --filter flag", func() {
		session := podmanTest.Podman([]string{"volume", "create", "--label", "foo=bar", "myvol"})
		volName := session.OutputToString()
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "label=foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToStringArray()[1]).To(ContainSubstring(volName))

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "label=foo=foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "label=foo=bar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToStringArray()[1]).To(ContainSubstring(volName))

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "label=foo=baz"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman ls volume with --filter until flag", func() {
		session := podmanTest.Podman([]string{"volume", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "until=5000000000"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(2))

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "until=50000"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman volume ls with --filter dangling", func() {
		volName1 := "volume1"
		session := podmanTest.Podman([]string{"volume", "create", volName1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		volName2 := "volume2"
		session2 := podmanTest.Podman([]string{"volume", "create", volName2})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())

		ctr := podmanTest.Podman([]string{"create", "-v", fmt.Sprintf("%s:/test", volName2), ALPINE, "sh"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		lsNoDangling := podmanTest.Podman([]string{"volume", "ls", "--filter", "dangling=false", "--quiet"})
		lsNoDangling.WaitWithDefaultTimeout()
		Expect(lsNoDangling).Should(ExitCleanly())
		Expect(lsNoDangling.OutputToString()).To(ContainSubstring(volName2))

		lsDangling := podmanTest.Podman([]string{"volume", "ls", "--filter", "dangling=true", "--quiet"})
		lsDangling.WaitWithDefaultTimeout()
		Expect(lsDangling).Should(ExitCleanly())
		Expect(lsDangling.OutputToString()).To(ContainSubstring(volName1))
	})

	It("podman ls volume with --filter name", func() {
		volName1 := "volume1"
		session := podmanTest.Podman([]string{"volume", "create", volName1})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		volName2 := "volume2"
		session2 := podmanTest.Podman([]string{"volume", "create", volName2})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "name=volume1*"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(3))
		Expect(session.OutputToStringArray()[1]).To(ContainSubstring(volName1))
		Expect(session.OutputToStringArray()[2]).To(ContainSubstring(volName2))

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "name=volumex"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls", "--filter", "name=volume1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToStringArray()[1]).To(ContainSubstring(volName1))
	})

	It("podman ls volume with multiple --filter flag", func() {
		session := podmanTest.Podman([]string{"volume", "create", "--label", "a=b", "--label", "b=c", "vol1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		vol1Name := session.OutputToString()

		session = podmanTest.Podman([]string{"volume", "create", "--label", "b=c", "--label", "a=b", "vol2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		vol2Name := session.OutputToString()

		session = podmanTest.Podman([]string{"volume", "create", "--label", "b=c", "--label", "c=d", "vol3"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		vol3Name := session.OutputToString()

		session = podmanTest.Podman([]string{"volume", "ls", "-q", "--filter", "label=a=b", "--filter", "label=b=c"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToStringArray()[0]).To(Equal(vol1Name))
		Expect(session.OutputToStringArray()[1]).To(Equal(vol2Name))

		session = podmanTest.Podman([]string{"volume", "ls", "-q", "--filter", "label=c=d", "--filter", "label=b=c"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(1))
		Expect(session.OutputToStringArray()[0]).To(Equal(vol3Name))
	})

	It("podman ls volume filters should combine with AND logic", func() {
		// Create volumes with different label combinations to test AND logic
		session := podmanTest.Podman([]string{"volume", "create", "--label", "a=b", "vol-with-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		volWithA := session.OutputToString()

		session = podmanTest.Podman([]string{"volume", "create", "--label", "c=d", "vol-with-c"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		volWithC := session.OutputToString()

		session = podmanTest.Podman([]string{"volume", "create", "--label", "a=b", "--label", "c=d", "vol-with-both"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		volWithBoth := session.OutputToString()

		// Sleep to ensure time difference for until filter
		time.Sleep(1100 * time.Millisecond)
		session = podmanTest.Podman([]string{"volume", "create", "--label", "a=b", "vol-new"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		volNew := session.OutputToString()

		// Test AND logic: both label=a=b AND label=c=d must match
		session = podmanTest.Podman([]string{"volume", "ls", "-q", "--filter", "label=a=b", "--filter", "label=c=d"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// Only vol-with-both should match when filters are combined with AND
		Expect(session.OutputToStringArray()).To(HaveLen(1))
		Expect(session.OutputToStringArray()[0]).To(Equal(volWithBoth))

		// Test AND logic: label=a=b AND label!=c=d must match
		session = podmanTest.Podman([]string{"volume", "ls", "-q", "--filter", "label=a=b", "--filter", "label!=c=d"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// Only volWithA and volNew should match (have a=b but not c=d)
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToStringArray()).To(ContainElement(volWithA))
		Expect(session.OutputToStringArray()).To(ContainElement(volNew))

		// Test that individual filters still work correctly
		session = podmanTest.Podman([]string{"volume", "ls", "-q", "--filter", "label=a=b"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// Should match all volumes with a=b label
		Expect(session.OutputToStringArray()).To(HaveLen(3))
		Expect(session.OutputToStringArray()).To(ContainElement(volWithA))
		Expect(session.OutputToStringArray()).To(ContainElement(volWithBoth))
		Expect(session.OutputToStringArray()).To(ContainElement(volNew))

		session = podmanTest.Podman([]string{"volume", "ls", "-q", "--filter", "label!=c=d"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// Should match all volumes without c=d label
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToStringArray()).To(ContainElement(volWithA))
		Expect(session.OutputToStringArray()).To(ContainElement(volNew))

		// Test filtering for volumes with c=d label
		session = podmanTest.Podman([]string{"volume", "ls", "-q", "--filter", "label=c=d"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// Should match volumes with c=d label
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToStringArray()).To(ContainElement(volWithC))
		Expect(session.OutputToStringArray()).To(ContainElement(volWithBoth))
	})

	It("podman ls volume filter within-key OR logic combined with cross-key AND logic", func() {
		// Test that values within the same filter key use OR, but different keys use AND
		session := podmanTest.Podman([]string{"volume", "create", "--label", "env=prod", "vol-prod"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		volProd := session.OutputToString()

		session = podmanTest.Podman([]string{"volume", "create", "--label", "env=dev", "vol-dev"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		volDev := session.OutputToString()

		session = podmanTest.Podman([]string{"volume", "create", "--label", "env=test", "vol-test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		volTest := session.OutputToString()

		session = podmanTest.Podman([]string{"volume", "create", "--label", "env=prod", "--label", "team=alpha", "vol-prod-alpha"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		volProdAlpha := session.OutputToString()

		session = podmanTest.Podman([]string{"volume", "create", "--label", "env=dev", "--label", "team=alpha", "vol-dev-alpha"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		volDevAlpha := session.OutputToString()

		// Test: env=(prod OR dev) AND team=alpha - should match volumes with team=alpha AND (env=prod OR env=dev)
		session = podmanTest.Podman([]string{"volume", "ls", "-q", "--filter", "label=env=prod", "--filter", "label=env=dev", "--filter", "label=team=alpha"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		result := session.OutputToStringArray()

		// Should match both volumes that have team=alpha AND have either env=prod OR env=dev
		Expect(result).To(HaveLen(2))
		Expect(result).To(ContainElement(volProdAlpha))
		Expect(result).To(ContainElement(volDevAlpha))
		// Should not match volumes without team=alpha, even if they have the right env
		Expect(result).NotTo(ContainElement(volProd))
		Expect(result).NotTo(ContainElement(volDev))
		Expect(result).NotTo(ContainElement(volTest))
	})

	It("podman ls volume with --filter since/after", func() {
		vol1 := "vol1"
		vol2 := "vol2"
		vol3 := "vol3"

		session := podmanTest.Podman([]string{"volume", "create", vol1})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "create", vol2})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "create", vol3})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls", "-q", "--filter", "since=" + vol1})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToStringArray()[0]).To(Equal(vol2))
		Expect(session.OutputToStringArray()[1]).To(Equal(vol3))

		session = podmanTest.Podman([]string{"volume", "ls", "-q", "--filter", "after=" + vol1})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToStringArray()).To(HaveLen(2))
		Expect(session.OutputToStringArray()[0]).To(Equal(vol2))
		Expect(session.OutputToStringArray()[1]).To(Equal(vol3))
	})
})
