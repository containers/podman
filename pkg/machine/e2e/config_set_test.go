package e2e_test

import (
	"strconv"
)

type setMachine struct {
	cpus     *uint
	diskSize *uint
	memory   *uint

	cmd []string
}

func (i *setMachine) buildCmd(m *machineTestBuilder) []string {
	cmd := []string{"machine", "set"}
	if i.cpus != nil {
		cmd = append(cmd, "--cpus", strconv.Itoa(int(*i.cpus)))
	}
	if i.diskSize != nil {
		cmd = append(cmd, "--disk-size", strconv.Itoa(int(*i.diskSize)))
	}
	if i.memory != nil {
		cmd = append(cmd, "--memory", strconv.Itoa(int(*i.memory)))
	}
	cmd = append(cmd, m.name)
	i.cmd = cmd
	return cmd
}

func (i *setMachine) withCPUs(num uint) *setMachine {
	i.cpus = &num
	return i
}
func (i *setMachine) withDiskSize(size uint) *setMachine {
	i.diskSize = &size
	return i
}

func (i *setMachine) withMemory(num uint) *setMachine {
	i.memory = &num
	return i
}
