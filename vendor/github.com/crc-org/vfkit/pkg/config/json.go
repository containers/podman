package config

import (
	"encoding/json"
	"fmt"
)

// The technique for json (de)serialization was explained here:
// http://gregtrowbridge.com/golang-json-serialization-with-interfaces/

type vmComponentKind string

const (
	// Bootloader kinds
	efiBootloader   vmComponentKind = "efiBootloader"
	linuxBootloader vmComponentKind = "linuxBootloader"

	// VirtIO device kinds
	vfNet          vmComponentKind = "virtionet"
	vfVsock        vmComponentKind = "virtiosock"
	vfBlk          vmComponentKind = "virtioblk"
	vfFs           vmComponentKind = "virtiofs"
	vfRng          vmComponentKind = "virtiorng"
	vfSerial       vmComponentKind = "virtioserial"
	vfGpu          vmComponentKind = "virtiogpu"
	vfInput        vmComponentKind = "virtioinput"
	usbMassStorage vmComponentKind = "usbmassstorage"
	rosetta        vmComponentKind = "rosetta"
)

type jsonKind struct {
	Kind vmComponentKind `json:"kind"`
}

func kind(k vmComponentKind) jsonKind {
	return jsonKind{Kind: k}
}

func unmarshalBootloader(rawMsg json.RawMessage) (Bootloader, error) {
	var (
		kind       jsonKind
		bootloader Bootloader
		err        error
	)
	if err := json.Unmarshal(rawMsg, &kind); err != nil {
		return nil, err
	}
	switch kind.Kind {
	case efiBootloader:
		var efi EFIBootloader
		err = json.Unmarshal(rawMsg, &efi)
		if err == nil {
			bootloader = &efi
		}
	case linuxBootloader:
		var linux LinuxBootloader
		err = json.Unmarshal(rawMsg, &linux)
		if err == nil {
			bootloader = &linux
		}
	default:
		err = fmt.Errorf("unknown 'kind' field: '%s'", kind)
	}

	return bootloader, err
}

func unmarshalDevices(rawMsg json.RawMessage) ([]VirtioDevice, error) {
	var (
		rawDevices []*json.RawMessage
		devices    []VirtioDevice
	)

	err := json.Unmarshal(rawMsg, &rawDevices)
	if err != nil {
		return nil, err
	}

	for _, msg := range rawDevices {
		dev, err := unmarshalDevice(*msg)
		if err != nil {
			return nil, err
		}
		devices = append(devices, dev)
	}

	return devices, nil
}

func unmarshalDevice(rawMsg json.RawMessage) (VirtioDevice, error) {
	var (
		kind jsonKind
		dev  VirtioDevice
		err  error
	)
	if err := json.Unmarshal(rawMsg, &kind); err != nil {
		return nil, err
	}
	switch kind.Kind {
	case vfNet:
		var newDevice VirtioNet
		err = json.Unmarshal(rawMsg, &newDevice)
		dev = &newDevice
	case vfVsock:
		var newDevice VirtioVsock
		err = json.Unmarshal(rawMsg, &newDevice)
		dev = &newDevice
	case vfBlk:
		var newDevice VirtioBlk
		err = json.Unmarshal(rawMsg, &newDevice)
		dev = &newDevice
	case vfFs:
		var newDevice VirtioFs
		err = json.Unmarshal(rawMsg, &newDevice)
		dev = &newDevice
	case rosetta:
		var newDevice RosettaShare
		err = json.Unmarshal(rawMsg, &newDevice)
		dev = &newDevice
	case vfRng:
		var newDevice VirtioRng
		err = json.Unmarshal(rawMsg, &newDevice)
		dev = &newDevice
	case vfSerial:
		var newDevice VirtioSerial
		err = json.Unmarshal(rawMsg, &newDevice)
		dev = &newDevice
	case vfGpu:
		var newDevice VirtioGPU
		err = json.Unmarshal(rawMsg, &newDevice)
		dev = &newDevice
	case vfInput:
		var newDevice VirtioInput
		err = json.Unmarshal(rawMsg, &newDevice)
		dev = &newDevice
	case usbMassStorage:
		var newDevice USBMassStorage
		err = json.Unmarshal(rawMsg, &newDevice)
		dev = &newDevice
	default:
		err = fmt.Errorf("unknown 'kind' field: '%s'", kind)
	}

	if err != nil {
		return nil, err
	}
	return dev, nil
}

// UnmarshalJSON is a custom deserializer for VirtualMachine.  The custom work
// is needed because VirtualMachine uses interfaces in its struct and JSON cannot
// determine which implementation of the interface to deserialize to.
func (vm *VirtualMachine) UnmarshalJSON(b []byte) error {
	var (
		err   error
		input map[string]*json.RawMessage
	)

	if err := json.Unmarshal(b, &input); err != nil {
		return err
	}

	for idx, rawMsg := range input {
		if rawMsg == nil {
			continue
		}
		switch idx {
		case "vcpus":
			err = json.Unmarshal(*rawMsg, &vm.Vcpus)
		case "memoryBytes":
			err = json.Unmarshal(*rawMsg, &vm.MemoryBytes)
		case "bootloader":
			var bootloader Bootloader
			bootloader, err = unmarshalBootloader(*rawMsg)
			if err == nil {
				vm.Bootloader = bootloader
			}
		case "timesync":
			err = json.Unmarshal(*rawMsg, &vm.Timesync)
		case "devices":
			var devices []VirtioDevice
			devices, err = unmarshalDevices(*rawMsg)
			if err == nil {
				vm.Devices = devices
			}
		}

		if err != nil {
			return err
		}
	}
	return nil
}

func (bootloader *EFIBootloader) MarshalJSON() ([]byte, error) {
	type blWithKind struct {
		jsonKind
		EFIBootloader
	}
	return json.Marshal(blWithKind{
		jsonKind:      kind(efiBootloader),
		EFIBootloader: *bootloader,
	})
}

func (bootloader *LinuxBootloader) MarshalJSON() ([]byte, error) {
	type blWithKind struct {
		jsonKind
		LinuxBootloader
	}
	return json.Marshal(blWithKind{
		jsonKind:        kind(linuxBootloader),
		LinuxBootloader: *bootloader,
	})
}

func (dev *VirtioNet) MarshalJSON() ([]byte, error) {
	type devWithKind struct {
		jsonKind
		VirtioNet
	}
	return json.Marshal(devWithKind{
		jsonKind:  kind(vfNet),
		VirtioNet: *dev,
	})
}

func (dev *VirtioVsock) MarshalJSON() ([]byte, error) {
	type devWithKind struct {
		jsonKind
		VirtioVsock
	}
	return json.Marshal(devWithKind{
		jsonKind:    kind(vfVsock),
		VirtioVsock: *dev,
	})
}

func (dev *VirtioBlk) MarshalJSON() ([]byte, error) {
	type devWithKind struct {
		jsonKind
		VirtioBlk
	}
	return json.Marshal(devWithKind{
		jsonKind:  kind(vfBlk),
		VirtioBlk: *dev,
	})
}

func (dev *VirtioFs) MarshalJSON() ([]byte, error) {
	type devWithKind struct {
		jsonKind
		VirtioFs
	}
	return json.Marshal(devWithKind{
		jsonKind: kind(vfFs),
		VirtioFs: *dev,
	})
}

func (dev *RosettaShare) MarshalJSON() ([]byte, error) {
	type devWithKind struct {
		jsonKind
		RosettaShare
	}
	return json.Marshal(devWithKind{
		jsonKind:     kind(rosetta),
		RosettaShare: *dev,
	})
}

func (dev *VirtioRng) MarshalJSON() ([]byte, error) {
	type devWithKind struct {
		jsonKind
		VirtioRng
	}
	return json.Marshal(devWithKind{
		jsonKind:  kind(vfRng),
		VirtioRng: *dev,
	})
}

func (dev *VirtioSerial) MarshalJSON() ([]byte, error) {
	type devWithKind struct {
		jsonKind
		VirtioSerial
	}
	return json.Marshal(devWithKind{
		jsonKind:     kind(vfSerial),
		VirtioSerial: *dev,
	})
}

func (dev *VirtioGPU) MarshalJSON() ([]byte, error) {
	type devWithKind struct {
		jsonKind
		VirtioGPU
	}
	return json.Marshal(devWithKind{
		jsonKind:  kind(vfGpu),
		VirtioGPU: *dev,
	})
}

func (dev *VirtioInput) MarshalJSON() ([]byte, error) {
	type devWithKind struct {
		jsonKind
		VirtioInput
	}
	return json.Marshal(devWithKind{
		jsonKind:    kind(vfInput),
		VirtioInput: *dev,
	})
}

func (dev *USBMassStorage) MarshalJSON() ([]byte, error) {
	type devWithKind struct {
		jsonKind
		USBMassStorage
	}
	return json.Marshal(devWithKind{
		jsonKind:       kind(usbMassStorage),
		USBMassStorage: *dev,
	})
}
