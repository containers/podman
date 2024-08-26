package e2e_test

import (
	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/define"
	jsoniter "github.com/json-iterator/go"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman inspect stop", func() {

	It("inspect two machines", func() {
		name1 := randomString()
		name2 := randomString()

		// inspect bad name
		inspectBad := inspectMachine{}
		reallyLongName := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		session, err := mb.setName(reallyLongName).setCmd(&inspectBad).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))

		i := new(initMachine)
		machine1, err := mb.setName(name1).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(machine1).To(Exit(0))

		ii := new(initMachine)
		machine2, err := mb.setName(name2).setCmd(ii.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(machine2).To(Exit(0))

		inspect := new(inspectMachine)
		inspect = inspect.withFormat("{{.Name}}")
		inspectSession, err := mb.setName(name1).setCmd(inspect).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.Bytes()).To(ContainSubstring(name1))

		// regular inspect should
		inspectJSON := new(inspectMachine)
		inspectSession, err = mb.setName(name1).setCmd(inspectJSON).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))

		var inspectInfo []machine.InspectInfo
		err = jsoniter.Unmarshal(inspectSession.Bytes(), &inspectInfo)
		Expect(err).ToNot(HaveOccurred())

		switch testProvider.VMType() {
		case define.HyperVVirt, define.WSLVirt:
			Expect(inspectInfo[0].ConnectionInfo.PodmanPipe.GetPath()).To(ContainSubstring("podman-"))
		default:
			Expect(inspectInfo[0].ConnectionInfo.PodmanSocket.GetPath()).To(HaveSuffix("api.sock"))
		}

		inspect = new(inspectMachine)
		inspect = inspect.withFormat("{{.Name}}")
		inspectSession, err = mb.setName(name1).setCmd(inspect).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.Bytes()).To(ContainSubstring(name1))

		// check invalid template returns error
		inspect = new(inspectMachine)
		inspect = inspect.withFormat("{{.Abcde}}")
		inspectSession, err = mb.setName(name1).setCmd(inspect).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(125))
		Expect(inspectSession.errorToString()).To(ContainSubstring("can't evaluate field Abcde in type machine.InspectInfo"))
	})

	It("inspect shows a unique socket name per machine", func() {
		skipIfVmtype(define.WSLVirt, "test is only relevant for Unix based providers")
		skipIfVmtype(define.HyperVVirt, "test is only relevant for Unix based machines")

		var socks []string
		for c := 0; c < 2; c++ {
			name := randomString()
			i := new(initMachine)
			session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath)).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(session).To(Exit(0))

			// regular inspect should
			inspectJSON := new(inspectMachine)
			inspectSession, err := mb.setName(name).setCmd(inspectJSON).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(inspectSession).To(Exit(0))

			var inspectInfo []machine.InspectInfo
			err = jsoniter.Unmarshal(inspectSession.Bytes(), &inspectInfo)
			Expect(err).ToNot(HaveOccurred())
			socks = append(socks, inspectInfo[0].ConnectionInfo.PodmanSocket.GetPath())
		}

		Expect(socks[0]).ToNot(Equal(socks[1]))
	})
})
