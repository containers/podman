package machine

import "fmt"

type CreateOptions struct {
	CPUS       uint64
	Memory     uint64
	KernelPath string
	Devices    []VMDevices
}

type VMDevices struct {
	Path     string
	ReadOnly bool
}

type VM interface {
	Create(name string, opts CreateOptions) error
	Start(name string) error
	Stop(name string) error
}

type TestVM struct {
}

func (vm *TestVM) Create(name string, opts CreateOptions) error {
	fmt.Printf("Created: %s\n", name)
	return nil
}

func (vm *TestVM) Start(name string) error {
	fmt.Printf("Started: %s\n", name)
	return nil
}
func (vm *TestVM) Stop(name string) error {
	fmt.Printf("Stopped: %s\n", name)
	return nil
}
