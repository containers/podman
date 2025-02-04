package e2e_test

import (
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

type initMachine struct {
	/*
			      --cpus uint              Number of CPUs (default 1)
			      --disk-size uint         Disk size in GiB (default 100)
			      --ignition-path string   Path to ignition file
			      --username string        Username of the remote user (default "core" for FCOS, "user" for Fedora)
			      --image-path string      Path to bootable image (default "testing")
			  -m, --memory uint            Memory in MiB (default 2048)
			      --now                    Start machine now
			      --rootful                Whether this machine should prefer rootful container execution
		          --playbook string        Run an ansible playbook after first boot
			      --timezone string        Set timezone (default "local")
			  -v, --volume stringArray     Volumes to mount, source:target
			      --volume-driver string   Optional volume driver

	*/
	playbook           string
	cpus               *uint
	diskSize           *uint
	ignitionPath       string
	username           string
	image              string
	memory             *uint
	now                bool
	timezone           string
	rootful            bool
	volumes            []string
	userModeNetworking bool

	cmd []string
}

func (i *initMachine) buildCmd(m *machineTestBuilder) []string {
	diskSize := defaultDiskSize
	cmd := []string{"machine", "init"}
	if i.cpus != nil {
		cmd = append(cmd, "--cpus", strconv.Itoa(int(*i.cpus)))
	}
	if i.diskSize != nil {
		diskSize = *i.diskSize
	}
	cmd = append(cmd, "--disk-size", strconv.Itoa(int(diskSize)))
	if l := len(i.ignitionPath); l > 0 {
		cmd = append(cmd, "--ignition-path", i.ignitionPath)
	}
	if l := len(i.username); l > 0 {
		cmd = append(cmd, "--username", i.username)
	}
	if l := len(i.image); l > 0 {
		cmd = append(cmd, "--image", i.image)
	}
	if i.memory != nil {
		cmd = append(cmd, "--memory", strconv.Itoa(int(*i.memory)))
	}
	if l := len(i.timezone); l > 0 {
		cmd = append(cmd, "--timezone", i.timezone)
	}
	for _, v := range i.volumes {
		cmd = append(cmd, "--volume", v)
	}
	if i.now {
		cmd = append(cmd, "--now")
	}
	if i.rootful {
		cmd = append(cmd, "--rootful")
	}
	if l := len(i.playbook); l > 0 {
		cmd = append(cmd, "--playbook", i.playbook)
	}
	if i.userModeNetworking {
		cmd = append(cmd, "--user-mode-networking")
	}
	name := m.name
	cmd = append(cmd, name)

	// when we create a new VM remove it again as cleanup
	DeferCleanup(func() {
		r := new(rmMachine)
		session, err := m.setName(name).setCmd(r.withForce()).run()
		Expect(err).ToNot(HaveOccurred(), "error occurred rm'ing machine")
		// Some test create a invalid VM so the VM does not exists in this case we have to ignore the error.
		// It would be much better if rm -f would behave like other commands and ignore not exists errors.
		if session.ExitCode() == 125 {
			if strings.Contains(session.errorToString(), "VM does not exist") {
				return
			}

			// FIXME:#24344 work-around for custom ignition removal
			if strings.Contains(session.errorToString(), "failed to remove machines files: unable to find connection named") {
				return
			}
		}
		Expect(session).To(Exit(0))
	})

	i.cmd = cmd
	return cmd
}

func (i *initMachine) withCPUs(num uint) *initMachine {
	i.cpus = &num
	return i
}
func (i *initMachine) withDiskSize(size uint) *initMachine {
	i.diskSize = &size
	return i
}

func (i *initMachine) withIgnitionPath(path string) *initMachine { //nolint:unused
	i.ignitionPath = path
	return i
}

func (i *initMachine) withUsername(username string) *initMachine {
	i.username = username
	return i
}

func (i *initMachine) withImage(path string) *initMachine {
	i.image = path
	return i
}

func (i *initMachine) withMemory(num uint) *initMachine {
	i.memory = &num
	return i
}

func (i *initMachine) withNow() *initMachine {
	i.now = true
	return i
}

func (i *initMachine) withTimezone(tz string) *initMachine {
	i.timezone = tz
	return i
}

func (i *initMachine) withVolume(v string) *initMachine {
	i.volumes = append(i.volumes, v)
	return i
}

func (i *initMachine) withRootful(r bool) *initMachine {
	i.rootful = r
	return i
}

func (i *initMachine) withRunPlaybook(p string) *initMachine {
	i.playbook = p
	return i
}

func (i *initMachine) withUserModeNetworking(r bool) *initMachine { //nolint:unused
	i.userModeNetworking = r
	return i
}
