package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman events", func() {
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
		SkipIfRootless()
		SkipIfNotFedora()
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		result := podmanTest.Podman([]string{"events", "--stream=false"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(BeZero())
	})

	It("podman events with an event filter", func() {
		SkipIfRootless()
		SkipIfNotFedora()
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		result := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "event=start"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray()) >= 1)
	})

	It("podman events with an event filter and container=cid", func() {
		Skip("Does not work on v2")
		SkipIfRootless()
		SkipIfNotFedora()
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		_, ec2, cid2 := podmanTest.RunLsContainer("")
		Expect(ec2).To(Equal(0))
		time.Sleep(5 * time.Second)
		result := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "event=start", "--filter", fmt.Sprintf("container=%s", cid)})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).To(Equal(1))
		Expect(!strings.Contains(result.OutputToString(), cid2))
	})

	It("podman events with a type and filter container=id", func() {
		SkipIfRootless()
		SkipIfNotFedora()
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		result := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "type=pod", "--filter", fmt.Sprintf("container=%s", cid)})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).To(Equal(0))
	})

	It("podman events with a type", func() {
		SkipIfRootless()
		SkipIfNotFedora()
		setup := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:foobarpod", ALPINE, "top"})
		setup.WaitWithDefaultTimeout()
		stop := podmanTest.Podman([]string{"pod", "stop", "foobarpod"})
		stop.WaitWithDefaultTimeout()
		Expect(stop.ExitCode()).To(Equal(0))
		Expect(setup.ExitCode()).To(Equal(0))
		result := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "type=pod", "--filter", "pod=foobarpod"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		fmt.Println(result.OutputToStringArray())
		Expect(len(result.OutputToStringArray()) >= 2)
	})

	It("podman events --since", func() {
		SkipIfRootless()
		SkipIfNotFedora()
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		result := podmanTest.Podman([]string{"events", "--stream=false", "--since", "1m"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(BeZero())
	})

	It("podman events --until", func() {
		SkipIfRootless()
		SkipIfNotFedora()
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		test := podmanTest.Podman([]string{"events", "--help"})
		test.WaitWithDefaultTimeout()
		fmt.Println(test.OutputToStringArray())
		result := podmanTest.Podman([]string{"events", "--stream=false", "--since", "1h"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(BeZero())
	})

	It("podman events format", func() {
		SkipIfRootless()
		SkipIfNotFedora()
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		test := podmanTest.Podman([]string{"events", "--stream=false", "--format", "json"})
		test.WaitWithDefaultTimeout()
		jsonArr := test.OutputToStringArray()
		Expect(len(jsonArr)).To(Not(BeZero()))
		eventsMap := make(map[string]string)
		err := json.Unmarshal([]byte(jsonArr[0]), &eventsMap)
		Expect(err).To(BeNil())
		_, exist := eventsMap["Status"]
		Expect(exist).To(BeTrue())
		Expect(test.ExitCode()).To(BeZero())

		test = podmanTest.Podman([]string{"events", "--stream=false", "--format", "{{json.}}"})
		test.WaitWithDefaultTimeout()
		jsonArr = test.OutputToStringArray()
		Expect(len(jsonArr)).To(Not(BeZero()))
		eventsMap = make(map[string]string)
		err = json.Unmarshal([]byte(jsonArr[0]), &eventsMap)
		Expect(err).To(BeNil())
		_, exist = eventsMap["Status"]
		Expect(exist).To(BeTrue())
		Expect(test.ExitCode()).To(BeZero())
	})
})
