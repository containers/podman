// +build !remoteclient

package integration

import (
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman generate systemd", func() {
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

	It("podman generate systemd on bogus container/pod", func() {
		session := podmanTest.Podman([]string{"generate", "systemd", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman generate systemd bad restart policy", func() {
		session := podmanTest.Podman([]string{"generate", "systemd", "--restart-policy", "never", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman generate systemd bad timeout value", func() {
		session := podmanTest.Podman([]string{"generate", "systemd", "--time", "-1", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman generate systemd good timeout value", func() {
		session := podmanTest.Podman([]string{"create", "--name", "foobar", "alpine", "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"generate", "systemd", "--time", "1234", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		found, _ := session.GrepString(" stop -t 1234 ")
		Expect(found).To(BeTrue())
	})

	It("podman generate systemd", func() {
		n := podmanTest.Podman([]string{"run", "--name", "nginx", "-dt", nginx})
		n.WaitWithDefaultTimeout()
		Expect(n.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"generate", "systemd", "nginx"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman generate systemd --files --name", func() {
		n := podmanTest.Podman([]string{"run", "--name", "nginx", "-dt", nginx})
		n.WaitWithDefaultTimeout()
		Expect(n.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"generate", "systemd", "--files", "--name", "nginx"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		for _, file := range session.OutputToStringArray() {
			os.Remove(file)
		}

		found, _ := session.GrepString("/container-nginx.service")
		Expect(found).To(BeTrue())
	})

	It("podman generate systemd with timeout", func() {
		n := podmanTest.Podman([]string{"run", "--name", "nginx", "-dt", nginx})
		n.WaitWithDefaultTimeout()
		Expect(n.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"generate", "systemd", "--time", "5", "nginx"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		found, _ := session.GrepString("podman stop -t 5")
		Expect(found).To(BeTrue())
	})

	It("podman generate systemd pod --name", func() {
		n := podmanTest.Podman([]string{"pod", "create", "--name", "foo"})
		n.WaitWithDefaultTimeout()
		Expect(n.ExitCode()).To(Equal(0))

		n = podmanTest.Podman([]string{"create", "--pod", "foo", "--name", "foo-1", "alpine", "top"})
		n.WaitWithDefaultTimeout()
		Expect(n.ExitCode()).To(Equal(0))

		n = podmanTest.Podman([]string{"create", "--pod", "foo", "--name", "foo-2", "alpine", "top"})
		n.WaitWithDefaultTimeout()
		Expect(n.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"generate", "systemd", "--time", "42", "--name", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Grepping the output (in addition to unit tests)
		found, _ := session.GrepString("# pod-foo.service")
		Expect(found).To(BeTrue())

		found, _ = session.GrepString("Requires=container-foo-1.service container-foo-2.service")
		Expect(found).To(BeTrue())

		found, _ = session.GrepString("# container-foo-1.service")
		Expect(found).To(BeTrue())

		found, _ = session.GrepString(" start foo-1")
		Expect(found).To(BeTrue())

		found, _ = session.GrepString("-infra") // infra container
		Expect(found).To(BeTrue())

		found, _ = session.GrepString("# container-foo-2.service")
		Expect(found).To(BeTrue())

		found, _ = session.GrepString(" stop -t 42 foo-2")
		Expect(found).To(BeTrue())

		found, _ = session.GrepString("BindsTo=pod-foo.service")
		Expect(found).To(BeTrue())

		found, _ = session.GrepString("PIDFile=")
		Expect(found).To(BeTrue())

		found, _ = session.GrepString("/userdata/conmon.pid")
		Expect(found).To(BeTrue())
	})

	It("podman generate systemd pod --name --files", func() {
		n := podmanTest.Podman([]string{"pod", "create", "--name", "foo"})
		n.WaitWithDefaultTimeout()
		Expect(n.ExitCode()).To(Equal(0))

		n = podmanTest.Podman([]string{"create", "--pod", "foo", "--name", "foo-1", "alpine", "top"})
		n.WaitWithDefaultTimeout()
		Expect(n.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"generate", "systemd", "--name", "--files", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		for _, file := range session.OutputToStringArray() {
			os.Remove(file)
		}

		found, _ := session.GrepString("/pod-foo.service")
		Expect(found).To(BeTrue())

		found, _ = session.GrepString("/container-foo-1.service")
		Expect(found).To(BeTrue())
	})

	It("podman generate systemd --new", func() {
		n := podmanTest.Podman([]string{"create", "--name", "foo", "alpine", "top"})
		n.WaitWithDefaultTimeout()
		Expect(n.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"generate", "systemd", "-t", "42", "--name", "--new", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Grepping the output (in addition to unit tests)
		found, _ := session.GrepString("# container-foo.service")
		Expect(found).To(BeTrue())

		found, _ = session.GrepString("stop --ignore --cidfile %t/%n-cid -t 42")
		Expect(found).To(BeTrue())
	})

	It("podman generate systemd --new without explicit detaching param", func() {
		n := podmanTest.Podman([]string{"create", "--name", "foo", "alpine", "top"})
		n.WaitWithDefaultTimeout()
		Expect(n.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"generate", "systemd", "--timeout", "42", "--name", "--new", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Grepping the output (in addition to unit tests)
		found, _ := session.GrepString("--cgroups=no-conmon -d")
		Expect(found).To(BeTrue())
	})

	It("podman generate systemd --new with explicit detaching param in middle", func() {
		n := podmanTest.Podman([]string{"create", "--name", "foo", "-d", "alpine", "top"})
		n.WaitWithDefaultTimeout()
		Expect(n.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"generate", "systemd", "--time", "42", "--name", "--new", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Grepping the output (in addition to unit tests)
		found, _ := session.GrepString("--name foo -d alpine top")
		Expect(found).To(BeTrue())
	})

	It("podman generate systemd --new pod", func() {
		n := podmanTest.Podman([]string{"pod", "create", "--name", "foo"})
		n.WaitWithDefaultTimeout()
		Expect(n.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"generate", "systemd", "--time", "42", "--name", "--new", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman generate systemd --container-prefix con", func() {
		n := podmanTest.Podman([]string{"create", "--name", "foo", "alpine", "top"})
		n.WaitWithDefaultTimeout()
		Expect(n.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"generate", "systemd", "--name", "--container-prefix", "con", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Grepping the output (in addition to unit tests)
		found, _ := session.GrepString("# con-foo.service")
		Expect(found).To(BeTrue())
	})

	It("podman generate systemd --separator _", func() {
		n := podmanTest.Podman([]string{"create", "--name", "foo", "alpine", "top"})
		n.WaitWithDefaultTimeout()
		Expect(n.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"generate", "systemd", "--name", "--separator", "_", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Grepping the output (in addition to unit tests)
		found, _ := session.GrepString("# container_foo.service")
		Expect(found).To(BeTrue())
	})

	It("podman generate systemd pod --pod-prefix p", func() {
		n := podmanTest.Podman([]string{"pod", "create", "--name", "foo"})
		n.WaitWithDefaultTimeout()
		Expect(n.ExitCode()).To(Equal(0))

		n = podmanTest.Podman([]string{"create", "--pod", "foo", "--name", "foo-1", "alpine", "top"})
		n.WaitWithDefaultTimeout()
		Expect(n.ExitCode()).To(Equal(0))

		n = podmanTest.Podman([]string{"create", "--pod", "foo", "--name", "foo-2", "alpine", "top"})
		n.WaitWithDefaultTimeout()
		Expect(n.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"generate", "systemd", "--pod-prefix", "p", "--name", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Grepping the output (in addition to unit tests)
		found, _ := session.GrepString("# p-foo.service")
		Expect(found).To(BeTrue())

		found, _ = session.GrepString("Requires=container-foo-1.service container-foo-2.service")
		Expect(found).To(BeTrue())

		found, _ = session.GrepString("# container-foo-1.service")
		Expect(found).To(BeTrue())

		found, _ = session.GrepString("BindsTo=p-foo.service")
		Expect(found).To(BeTrue())
	})

	It("podman generate systemd pod --pod-prefix p --container-prefix con --separator _ change all prefixes/separator", func() {
		n := podmanTest.Podman([]string{"pod", "create", "--name", "foo"})
		n.WaitWithDefaultTimeout()
		Expect(n.ExitCode()).To(Equal(0))

		n = podmanTest.Podman([]string{"create", "--pod", "foo", "--name", "foo-1", "alpine", "top"})
		n.WaitWithDefaultTimeout()
		Expect(n.ExitCode()).To(Equal(0))

		n = podmanTest.Podman([]string{"create", "--pod", "foo", "--name", "foo-2", "alpine", "top"})
		n.WaitWithDefaultTimeout()
		Expect(n.ExitCode()).To(Equal(0))

		session := podmanTest.Podman([]string{"generate", "systemd", "--container-prefix", "con", "--pod-prefix", "p", "--separator", "_", "--name", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		// Grepping the output (in addition to unit tests)
		found, _ := session.GrepString("# p_foo.service")
		Expect(found).To(BeTrue())

		found, _ = session.GrepString("Requires=con_foo-1.service con_foo-2.service")
		Expect(found).To(BeTrue())

		found, _ = session.GrepString("# con_foo-1.service")
		Expect(found).To(BeTrue())

		found, _ = session.GrepString("# con_foo-2.service")
		Expect(found).To(BeTrue())

		found, _ = session.GrepString("BindsTo=p_foo.service")
		Expect(found).To(BeTrue())
	})
})
