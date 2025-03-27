package e2e_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/containers/podman/v5/pkg/machine"
	"github.com/docker/docker/client"
	"github.com/goccy/go-json"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

const (
	NamedPipeProto = "npipe://"
)

var _ = Describe("run podman API test calls", func() {
	It("client connect to machine socket", func() {
		if runtime.GOOS == "windows" {
			Skip("Go docker client doesn't support unix socket on Windows")
		}
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		inspectJSON := new(inspectMachine)
		inspectSession, err := mb.setName(name).setCmd(inspectJSON).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))

		var inspectInfo []machine.InspectInfo
		err = json.Unmarshal(inspectSession.Bytes(), &inspectInfo)
		Expect(err).ToNot(HaveOccurred())
		sockPath := inspectInfo[0].ConnectionInfo.PodmanSocket.GetPath()

		cli, err := client.NewClientWithOpts(client.WithHost("unix://" + sockPath))
		Expect(err).ToNot(HaveOccurred())
		_, err = cli.Ping(context.Background())
		Expect(err).ToNot(HaveOccurred())
	})

	It("client connect to machine named pipe", func() {
		if runtime.GOOS != "windows" {
			Skip("test is only supported on Windows")
		}
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		inspectJSON := new(inspectMachine)
		inspectSession, err := mb.setName(name).setCmd(inspectJSON).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))

		var inspectInfo []machine.InspectInfo
		err = json.Unmarshal(inspectSession.Bytes(), &inspectInfo)
		Expect(err).ToNot(HaveOccurred())
		pipePath := inspectInfo[0].ConnectionInfo.PodmanPipe.GetPath()

		cli, err := client.NewClientWithOpts(client.WithHost(NamedPipeProto + filepath.ToSlash(pipePath)))
		Expect(err).ToNot(HaveOccurred())
		_, err = cli.Ping(context.Background())
		Expect(err).ToNot(HaveOccurred())
	})

	It("curl connect to machine socket", func() {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		inspectJSON := new(inspectMachine)
		inspectSession, err := mb.setName(name).setCmd(inspectJSON).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))

		var inspectInfo []machine.InspectInfo
		err = json.Unmarshal(inspectSession.Bytes(), &inspectInfo)
		Expect(err).ToNot(HaveOccurred())
		sockPath := inspectInfo[0].ConnectionInfo.PodmanSocket.GetPath()

		cmd := exec.Command("curl", "--unix-socket", sockPath, "http://d/v5.0.0/libpod/info")
		err = cmd.Run()
		Expect(err).ToNot(HaveOccurred())
	})
})
