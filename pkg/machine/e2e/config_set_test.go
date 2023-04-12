package e2e_test

import (
	"strconv"
)

type setMachine struct {
	cpus          *uint
	diskSize      *uint
	memory        *uint
	extraDiskNum  *uint
	extraDiskSize *uint

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
	if i.extraDiskNum != nil {
		cmd = append(cmd, "--extra-disk-num", strconv.Itoa(int(*i.extraDiskNum)))
	}
	if i.extraDiskSize != nil {
		cmd = append(cmd, "--extra-disk-size", strconv.Itoa(int(*i.extraDiskSize)))
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

func (i *setMachine) withExtraDiskNum(num uint) *setMachine {
	i.extraDiskNum = &num
	return i
}

func (i *setMachine) withExtraDiskSize(num uint) *setMachine {
	i.extraDiskSize = &num
	return i
}
