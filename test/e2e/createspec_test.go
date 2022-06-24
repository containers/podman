package integration

import (
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var jsonSpec = `{
    "httpproxy":true,
    "annotations":{
        "io.kubernetes.cri-o.TTY":"false"
    },
    "stop_timeout":10,
    "log_configuration":{
        "driver":"journald"
    },
    "raw_image_name":"alpine",
    "systemd":"true",
    "sdnotifyMode":"container",
    "pidns":{
    },
    "utsns":{
    },
    "containerCreateCommand":[
        "bin/podman",
        "create",
        "--cpus=5",
        "alpine",
        "ls",
        "/sys/fs"
    ],
    "init_container_type":"",
    "manage_password":true,
    "image":"alpine",
    "image_volume_mode":"anonymous",
    "ipcns":{
    },
    "seccomp_policy":"default",
    "userns":{
    },
    "idmappings":{
        "HostUIDMapping":true,
        "HostGIDMapping":true,
        "UIDMap":null,
        "GIDMap":null,
        "AutoUserNs":false,
        "AutoUserNsOpts":{
            "Size":0,
            "InitialSize":0,
            "PasswdFile":"",
            "GroupFile":"",
            "AdditionalUIDMappings":null,
            "AdditionalGIDMappings":null
        }
    },
    "umask":"0022",
    "cgroupns":{
    },
    "cgroups_mode":"enabled",
    "netns":{
    },
    "Networks":null,
    "use_image_hosts":false,
    "resource_limits":{
        "cpu":{
            "quota":500000,
            "period":100000
        }
    }

}`

var sparseSpec = `{
    "image":"alpine",
    "systemd":"true",
    "sdnotifyMode":"container",
	"cgroups_mode":"enabled",
	"resource_limits":{
        "cpu":{
            "quota":500000,
            "period":100000
        }
    }
}
`

var _ = Describe("Podman createspec", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		Expect(err).To(BeNil())
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman createspec basic test", func() {
		f, err := os.CreateTemp(tempdir, "podman")
		Expect(err).Should(BeNil())

		defer func() {
			err := os.Remove(f.Name())
			Expect(err).Should(BeNil())
		}()

		_, err = f.WriteString(jsonSpec)
		Expect(err).Should(BeNil())

		session := podmanTest.Podman([]string{"container", "createspec", f.Name()})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("podman createspec override from command line", func() {
		f, err := os.CreateTemp(tempdir, "podman")
		Expect(err).Should(BeNil())

		defer func() {
			err := os.Remove(f.Name())
			Expect(err).Should(BeNil())
		}()

		_, err = f.WriteString(jsonSpec)
		Expect(err).Should(BeNil())

		session := podmanTest.Podman([]string{"container", "createspec", "--start", "--cpus=2", f.Name()})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"container", "inspect", session.OutputToString()})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		out := session.InspectContainerToJSON()
		// override successful
		Expect(out[0].HostConfig.CpuQuota).Should(Equal(int64(200000)))
	})

	It("podman createspec sparse spec should work", func() {
		f, err := os.CreateTemp(tempdir, "podman")
		Expect(err).Should(BeNil())

		defer func() {
			err := os.Remove(f.Name())
			Expect(err).Should(BeNil())
		}()

		_, err = f.WriteString(sparseSpec)
		Expect(err).Should(BeNil())

		session := podmanTest.Podman([]string{"container", "createspec", "--start", f.Name()})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})
})
