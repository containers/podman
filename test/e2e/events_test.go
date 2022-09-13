package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/containers/podman/v3/libpod/events"
	. "github.com/containers/podman/v3/test/utils"
	"github.com/containers/storage/pkg/stringid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman events", func() {
	var (
		tempdir    string
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		var err error
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
		podmanTest.SeedImages()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)
	})

	// For most, all, of these tests we do not "live" test following a log because it may make a fragile test
	// system more complex.  Instead we run the "events" and then verify that the events are processed correctly.
	// Perhaps a future version of this test would put events in a go func and send output back over a channel
	// while events occur.

	// These tests are only known to work on Fedora ATM.  Other distributions
	// will be skipped.
	It("podman events", func() {
		SkipIfNotFedora()
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		result := podmanTest.Podman([]string{"events", "--stream=false"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("podman events with an event filter", func() {
		SkipIfNotFedora()
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		result := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "event=start"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(len(result.OutputToStringArray()) >= 1)
	})

	It("podman events with an event filter and container=cid", func() {
		Skip("Does not work on v2")
		SkipIfNotFedora()
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		_, ec2, cid2 := podmanTest.RunLsContainer("")
		Expect(ec2).To(Equal(0))
		time.Sleep(5 * time.Second)
		result := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "event=start", "--filter", fmt.Sprintf("container=%s", cid)})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(len(result.OutputToStringArray())).To(Equal(1))
		Expect(!strings.Contains(result.OutputToString(), cid2))
	})

	It("podman events with a type and filter container=id", func() {
		SkipIfNotFedora()
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		result := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "type=pod", "--filter", fmt.Sprintf("container=%s", cid)})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(len(result.OutputToStringArray())).To(Equal(0))
	})

	It("podman events with a type", func() {
		SkipIfNotFedora()
		setup := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:foobarpod", ALPINE, "top"})
		setup.WaitWithDefaultTimeout()
		stop := podmanTest.Podman([]string{"pod", "stop", "foobarpod"})
		stop.WaitWithDefaultTimeout()
		Expect(stop).Should(Exit(0))
		Expect(setup).Should(Exit(0))
		result := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "type=pod", "--filter", "pod=foobarpod"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		fmt.Println(result.OutputToStringArray())
		Expect(len(result.OutputToStringArray()) >= 2)
	})

	It("podman events --since", func() {
		SkipIfNotFedora()
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		result := podmanTest.Podman([]string{"events", "--stream=false", "--since", "1m"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("podman events --until", func() {
		SkipIfNotFedora()
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		result := podmanTest.Podman([]string{"events", "--stream=false", "--until", "1h"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
	})

	It("podman events format", func() {
		SkipIfNotFedora()
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))

		test := podmanTest.Podman([]string{"events", "--stream=false", "--format", "json"})
		test.WaitWithDefaultTimeout()
		Expect(test).To(Exit(0))

		jsonArr := test.OutputToStringArray()
		Expect(test.OutputToStringArray()).ShouldNot(BeEmpty())

		event := events.Event{}
		err := json.Unmarshal([]byte(jsonArr[0]), &event)
		Expect(err).ToNot(HaveOccurred())

		test = podmanTest.Podman([]string{"events", "--stream=false", "--format", "{{json.}}"})
		test.WaitWithDefaultTimeout()
		Expect(test).To(Exit(0))

		jsonArr = test.OutputToStringArray()
		Expect(test.OutputToStringArray()).ShouldNot(BeEmpty())

		event = events.Event{}
		err = json.Unmarshal([]byte(jsonArr[0]), &event)
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman events --until future", func() {
		name1 := stringid.GenerateRandomID()
		name2 := stringid.GenerateRandomID()
		name3 := stringid.GenerateRandomID()
		session := podmanTest.Podman([]string{"create", "--name", name1, ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer GinkgoRecover()
			defer wg.Done()

			// wait 2 seconds to be sure events is running
			time.Sleep(time.Second * 2)
			session = podmanTest.Podman([]string{"create", "--name", name2, ALPINE})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
			session = podmanTest.Podman([]string{"create", "--name", name3, ALPINE})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
		}()

		// unix timestamp in 10 seconds
		until := time.Now().Add(time.Second * 10).Unix()
		result := podmanTest.Podman([]string{"events", "--since", "30s", "--until", fmt.Sprint(until)})
		result.Wait(11)
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring(name1))
		Expect(result.OutputToString()).To(ContainSubstring(name2))
		Expect(result.OutputToString()).To(ContainSubstring(name3))

		// string duration in 10 seconds
		untilT := time.Now().Add(time.Second * 9)
		result = podmanTest.Podman([]string{"events", "--since", "30s", "--until", "10s"})
		result.Wait(11)
		Expect(result).Should(Exit(0))
		tEnd := time.Now()
		outDur := tEnd.Sub(untilT)
		diff := outDur.Seconds() > 0
		Expect(diff).To(Equal(true))
		Expect(result.OutputToString()).To(ContainSubstring(name1))
		Expect(result.OutputToString()).To(ContainSubstring(name2))
		Expect(result.OutputToString()).To(ContainSubstring(name3))

		wg.Wait()
	})
})
