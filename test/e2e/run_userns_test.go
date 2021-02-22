package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"strings"

	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman UserNS support", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		if os.Getenv("SKIP_USERNS") != "" {
			Skip("Skip userns tests.")
		}
		if _, err := os.Stat("/proc/self/uid_map"); err != nil {
			Skip("User namespaces not supported.")
		}
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

	It("podman uidmapping and gidmapping", func() {
		session := podmanTest.Podman([]string{"run", "--uidmap=0:100:5000", "--gidmap=0:200:5000", "alpine", "echo", "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ok, _ := session.GrepString("hello")
		Expect(ok).To(BeTrue())
	})

	// It essentially repeats the test above but with the `-it` short option
	// that broke execution at:
	//     https://github.com/containers/podman/pull/1066#issuecomment-403562116
	// To avoid a potential future regression, use this as a test.
	It("podman uidmapping and gidmapping with short-opts", func() {
		session := podmanTest.Podman([]string{"run", "--uidmap=0:1:5000", "--gidmap=0:200:5000", "-it", "alpine", "echo", "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ok, _ := session.GrepString("hello")
		Expect(ok).To(BeTrue())
	})

	It("podman uidmapping and gidmapping with a volume", func() {
		session := podmanTest.Podman([]string{"run", "--uidmap=0:1:500", "--gidmap=0:200:5000", "-v", "my-foo-volume:/foo:Z", "alpine", "echo", "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ok, _ := session.GrepString("hello")
		Expect(ok).To(BeTrue())
	})

	It("podman uidmapping and gidmapping --net=host", func() {
		session := podmanTest.Podman([]string{"run", "--net=host", "--uidmap=0:1:5000", "--gidmap=0:200:5000", "alpine", "echo", "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ok, _ := session.GrepString("hello")
		Expect(ok).To(BeTrue())
	})

	It("podman --userns=keep-id", func() {
		session := podmanTest.Podman([]string{"run", "--userns=keep-id", "alpine", "id", "-u"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		uid := fmt.Sprintf("%d", os.Geteuid())
		ok, _ := session.GrepString(uid)
		Expect(ok).To(BeTrue())
	})

	It("podman --userns=keep-id check passwd", func() {
		session := podmanTest.Podman([]string{"run", "--userns=keep-id", "alpine", "id", "-un"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		u, err := user.Current()
		Expect(err).To(BeNil())
		ok, _ := session.GrepString(u.Name)
		Expect(ok).To(BeTrue())
	})

	It("podman --userns=keep-id root owns /usr", func() {
		session := podmanTest.Podman([]string{"run", "--userns=keep-id", "alpine", "stat", "-c%u", "/usr"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("0"))
	})

	It("podman --userns=keep-id --user root:root", func() {
		session := podmanTest.Podman([]string{"run", "--userns=keep-id", "--user", "root:root", "alpine", "id", "-u"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(Equal("0"))
	})

	It("podman run --userns=keep-id can add users", func() {
		if os.Geteuid() == 0 {
			Skip("Test only runs without root")
		}

		userName := os.Getenv("USER")
		if userName == "" {
			Skip("Can't complete test if no username available")
		}

		ctrName := "ctr-name"
		session := podmanTest.Podman([]string{"run", "--userns=keep-id", "--user", "root:root", "-d", "--stop-signal", "9", "--name", ctrName, fedoraMinimal, "sleep", "600"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		exec1 := podmanTest.Podman([]string{"exec", "-t", "-i", ctrName, "cat", "/etc/passwd"})
		exec1.WaitWithDefaultTimeout()
		Expect(exec1.ExitCode()).To(Equal(0))
		Expect(exec1.OutputToString()).To(ContainSubstring(userName))

		exec2 := podmanTest.Podman([]string{"exec", "-t", "-i", ctrName, "useradd", "testuser"})
		exec2.WaitWithDefaultTimeout()
		Expect(exec2.ExitCode()).To(Equal(0))
	})

	It("podman --userns=auto", func() {
		u, err := user.Current()
		Expect(err).To(BeNil())
		name := u.Name
		if name == "root" {
			name = "containers"
		}

		content, err := ioutil.ReadFile("/etc/subuid")
		if err != nil {
			Skip("cannot read /etc/subuid")
		}
		if !strings.Contains(string(content), name) {
			Skip("cannot find mappings for the current user")
		}

		m := make(map[string]string)
		for i := 0; i < 5; i++ {
			session := podmanTest.Podman([]string{"run", "--userns=auto", "alpine", "cat", "/proc/self/uid_map"})
			session.WaitWithDefaultTimeout()
			Expect(session.ExitCode()).To(Equal(0))
			l := session.OutputToString()
			Expect(strings.Contains(l, "1024")).To(BeTrue())
			m[l] = l
		}
		// check for no duplicates
		Expect(len(m)).To(Equal(5))
	})

	It("podman --userns=auto:size=%d", func() {
		u, err := user.Current()
		Expect(err).To(BeNil())

		name := u.Name
		if name == "root" {
			name = "containers"
		}

		content, err := ioutil.ReadFile("/etc/subuid")
		if err != nil {
			Skip("cannot read /etc/subuid")
		}
		if !strings.Contains(string(content), name) {
			Skip("cannot find mappings for the current user")
		}

		session := podmanTest.Podman([]string{"run", "--userns=auto:size=500", "alpine", "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ok, _ := session.GrepString("500")

		session = podmanTest.Podman([]string{"run", "--userns=auto:size=3000", "alpine", "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ok, _ = session.GrepString("3000")

		session = podmanTest.Podman([]string{"run", "--userns=auto", "--user=2000:3000", "alpine", "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ok, _ = session.GrepString("3001")

		session = podmanTest.Podman([]string{"run", "--userns=auto", "--user=4000:1000", "alpine", "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ok, _ = session.GrepString("4001")
		Expect(ok).To(BeTrue())
	})

	It("podman --userns=auto:uidmapping=", func() {
		u, err := user.Current()
		Expect(err).To(BeNil())

		name := u.Name
		if name == "root" {
			name = "containers"
		}

		content, err := ioutil.ReadFile("/etc/subuid")
		if err != nil {
			Skip("cannot read /etc/subuid")
		}
		if !strings.Contains(string(content), name) {
			Skip("cannot find mappings for the current user")
		}

		session := podmanTest.Podman([]string{"run", "--userns=auto:uidmapping=0:0:1", "alpine", "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		output := session.OutputToString()
		Expect(output).To(MatchRegexp("\\s0\\s0\\s1"))

		session = podmanTest.Podman([]string{"run", "--userns=auto:size=8192,uidmapping=0:0:1", "alpine", "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ok, _ := session.GrepString("8191")
		Expect(ok).To(BeTrue())
	})

	It("podman --userns=auto:gidmapping=", func() {
		u, err := user.Current()
		Expect(err).To(BeNil())

		name := u.Name
		if name == "root" {
			name = "containers"
		}

		content, err := ioutil.ReadFile("/etc/subuid")
		if err != nil {
			Skip("cannot read /etc/subuid")
		}
		if !strings.Contains(string(content), name) {
			Skip("cannot find mappings for the current user")
		}

		session := podmanTest.Podman([]string{"run", "--userns=auto:gidmapping=0:0:1", "alpine", "cat", "/proc/self/gid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		output := session.OutputToString()
		Expect(output).To(MatchRegexp("\\s0\\s0\\s1"))

		session = podmanTest.Podman([]string{"run", "--userns=auto:size=8192,gidmapping=0:0:1", "alpine", "cat", "/proc/self/gid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		ok, _ := session.GrepString("8191")
		Expect(ok).To(BeTrue())
	})

	It("podman --userns=container:CTR", func() {
		ctrName := "userns-ctr"
		session := podmanTest.Podman([]string{"run", "-d", "--uidmap=0:0:1", "--uidmap=1:1:4998", "--name", ctrName, "alpine", "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// runc has an issue and we also need to join the IPC namespace.
		session = podmanTest.Podman([]string{"run", "--rm", "--userns=container:" + ctrName, "--ipc=container:" + ctrName, "alpine", "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		ok, _ := session.GrepString("4998")
		Expect(ok).To(BeTrue())

		session = podmanTest.Podman([]string{"run", "--rm", "--userns=container:" + ctrName, "--net=container:" + ctrName, "alpine", "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		ok, _ = session.GrepString("4998")
		Expect(ok).To(BeTrue())
	})

	It("podman --user with volume", func() {
		tests := []struct {
			uid, gid, arg, vol string
		}{
			{"0", "0", "0:0", "vol-0"},
			{"1000", "0", "1000", "vol-1"},
			{"1000", "1000", "1000:1000", "vol-2"},
		}

		for _, tt := range tests {
			session := podmanTest.Podman([]string{"run", "-d", "--user", tt.arg, "--mount", "type=volume,src=" + tt.vol + ",dst=/home/user", "alpine", "top"})
			session.WaitWithDefaultTimeout()
			Expect(session.ExitCode()).To(Equal(0))

			inspectUID := podmanTest.Podman([]string{"volume", "inspect", "--format", "{{ .UID }}", tt.vol})
			inspectUID.WaitWithDefaultTimeout()
			Expect(inspectUID.ExitCode()).To(Equal(0))
			Expect(inspectUID.OutputToString()).To(Equal(tt.uid))

			// Make sure we're defaulting to 0.
			inspectGID := podmanTest.Podman([]string{"volume", "inspect", "--format", "{{ .GID }}", tt.vol})
			inspectGID.WaitWithDefaultTimeout()
			Expect(inspectGID.ExitCode()).To(Equal(0))
			Expect(inspectGID.OutputToString()).To(Equal(tt.gid))
		}
	})
})
