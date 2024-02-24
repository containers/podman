//go:build windows

package vsock

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/Microsoft/go-winio"
	"github.com/containers/podman/v5/pkg/machine/sockets"
	"github.com/containers/podman/v5/utils"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows/registry"
)

var ErrVSockRegistryEntryExists = errors.New("registry entry already exists")

const (
	// HvsockMachineName is the string identifier for the machine name in a registry entry
	HvsockMachineName = "MachineName"
	// HvsockPurpose is the string identifier for the sock purpose in a registry entry
	HvsockPurpose = "Purpose"
	// VsockRegistryPath describes the registry path to where the hvsock registry entries live
	VsockRegistryPath = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Virtualization\GuestCommunicationServices`
	// LinuxVM is the default guid for a Linux VM on Windows
	LinuxVM = "FACB-11E6-BD58-64006A7986D3"
)

// HVSockPurpose describes what the hvsock is needed for
type HVSockPurpose int

const (
	// Network implies the sock is used for user-mode networking
	Network HVSockPurpose = iota
	// Events implies the sock is used for notification (like "Ready")
	Events
	// Fileserver implies that the sock is used for serving files from host to VM
	Fileserver
)

func (hv HVSockPurpose) string() string {
	switch hv {
	case Network:
		return "Network"
	case Events:
		return "Events"
	case Fileserver:
		return "Fileserver"
	}
	return ""
}

func (hv HVSockPurpose) Equal(purpose string) bool {
	return hv.string() == purpose
}

func toHVSockPurpose(p string) (HVSockPurpose, error) {
	switch p {
	case "Network":
		return Network, nil
	case "Events":
		return Events, nil
	case "Fileserver":
		return Fileserver, nil
	}
	return 0, fmt.Errorf("unknown hvsockpurpose: %s", p)
}

func openVSockRegistryEntry(entry string) (registry.Key, error) {
	return registry.OpenKey(registry.LOCAL_MACHINE, entry, registry.QUERY_VALUE)
}

// HVSockRegistryEntry describes a registry entry used in Windows for HVSOCK implementations
type HVSockRegistryEntry struct {
	KeyName     string        `json:"key_name"`
	Purpose     HVSockPurpose `json:"purpose"`
	Port        uint64        `json:"port"`
	MachineName string        `json:"machineName"`
	Key         registry.Key  `json:"key,omitempty"`
}

// Add creates a new Windows registry entry with string values from the
// HVSockRegistryEntry.
func (hv *HVSockRegistryEntry) Add() error {
	if err := hv.validate(); err != nil {
		return err
	}
	exists, err := hv.exists()
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("%q: %s", ErrVSockRegistryEntryExists, hv.KeyName)
	}
	parentKey, err := registry.OpenKey(registry.LOCAL_MACHINE, VsockRegistryPath, registry.QUERY_VALUE)
	defer func() {
		if err := parentKey.Close(); err != nil {
			logrus.Error(err)
		}
	}()
	if err != nil {
		return err
	}
	newKey, _, err := registry.CreateKey(parentKey, hv.KeyName, registry.WRITE)
	defer func() {
		if err := newKey.Close(); err != nil {
			logrus.Error(err)
		}
	}()
	if err != nil {
		return err
	}

	if err := newKey.SetStringValue(HvsockPurpose, hv.Purpose.string()); err != nil {
		return err
	}
	return newKey.SetStringValue(HvsockMachineName, hv.MachineName)
}

// Remove deletes the registry key and its string values
func (hv *HVSockRegistryEntry) Remove() error {
	return registry.DeleteKey(registry.LOCAL_MACHINE, hv.fqPath())
}

func (hv *HVSockRegistryEntry) fqPath() string {
	return fmt.Sprintf("%s\\%s", VsockRegistryPath, hv.KeyName)
}

func (hv *HVSockRegistryEntry) validate() error {
	if hv.Port < 1 {
		return errors.New("port must be larger than 1")
	}
	if len(hv.Purpose.string()) < 1 {
		return errors.New("required field purpose is empty")
	}
	if len(hv.MachineName) < 1 {
		return errors.New("required field machinename is empty")
	}
	if len(hv.KeyName) < 1 {
		return errors.New("required field keypath is empty")
	}
	return nil
}

func (hv *HVSockRegistryEntry) exists() (bool, error) {
	foo := hv.fqPath()
	_ = foo
	_, err := openVSockRegistryEntry(hv.fqPath())
	if err == nil {
		return true, nil
	}
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// findOpenHVSockPort looks for an open random port. it verifies the port is not
// already being used by another hvsock in the Windows registry.
func findOpenHVSockPort() (uint64, error) {
	// If we cannot find a free port in 10 attempts, something is wrong
	for i := 0; i < 10; i++ {
		port, err := utils.GetRandomPort()
		if err != nil {
			return 0, err
		}
		// Try and load registry entries by port to see if they exist
		_, err = LoadHVSockRegistryEntry(uint64(port))
		if err == nil {
			// the port is no good, it is being used; try again
			logrus.Errorf("port %d is already used for hvsock", port)
			continue
		}
		if errors.Is(err, registry.ErrNotExist) {
			// the port is good to go
			return uint64(port), nil
		}
		if err != nil {
			// something went wrong
			return 0, err
		}
	}
	return 0, errors.New("unable to find a free port for hvsock use")
}

// NewHVSockRegistryEntry is a constructor to make a new registry entry in Windows.  After making the new
// object, you must call the add() method to *actually* add it to the Windows registry.
func NewHVSockRegistryEntry(machineName string, purpose HVSockPurpose) (*HVSockRegistryEntry, error) {
	// a so-called wildcard entry ... everything from FACB -> 6D3 is MS special sauce
	// for a " linux vm".  this first segment is hexi for the hvsock port number
	// 00000400-FACB-11E6-BD58-64006A7986D3
	port, err := findOpenHVSockPort()
	if err != nil {
		return nil, err
	}
	r := HVSockRegistryEntry{
		KeyName:     portToKeyName(port),
		Purpose:     purpose,
		Port:        port,
		MachineName: machineName,
	}
	if err := r.Add(); err != nil {
		return nil, err
	}
	return &r, nil
}

func portToKeyName(port uint64) string {
	// this could be flattened but given the complexity, I thought it might
	// be more difficult to read
	hexi := strings.ToUpper(fmt.Sprintf("%08x", port))
	return fmt.Sprintf("%s-%s", hexi, LinuxVM)
}

func LoadHVSockRegistryEntry(port uint64) (*HVSockRegistryEntry, error) {
	keyName := portToKeyName(port)
	fqPath := fmt.Sprintf("%s\\%s", VsockRegistryPath, keyName)
	k, err := openVSockRegistryEntry(fqPath)
	if err != nil {
		return nil, err
	}
	p, _, err := k.GetStringValue(HvsockPurpose)
	if err != nil {
		return nil, err
	}

	purpose, err := toHVSockPurpose(p)
	if err != nil {
		return nil, err
	}

	machineName, _, err := k.GetStringValue(HvsockMachineName)
	if err != nil {
		return nil, err
	}
	return &HVSockRegistryEntry{
		KeyName:     keyName,
		Purpose:     purpose,
		Port:        port,
		MachineName: machineName,
		Key:         k,
	}, nil
}

// Listener returns a net.Listener for the given HvSock.
func (hv *HVSockRegistryEntry) Listener() (net.Listener, error) {
	n := winio.HvsockAddr{
		VMID:      winio.HvsockGUIDWildcard(), // When listening on the host side, use equiv of 0.0.0.0
		ServiceID: winio.VsockServiceID(uint32(hv.Port)),
	}
	listener, err := winio.ListenHvsock(&n)
	if err != nil {
		return nil, err
	}

	return listener, nil
}

// ListenSetupWait creates an hvsock on the windows side and returns
// a wait function that, when called, blocks until it receives a ready
// notification on the vsock
func (hv *HVSockRegistryEntry) ListenSetupWait() (func() error, io.Closer, error) {
	listener, err := hv.Listener()
	if err != nil {
		return nil, nil, err
	}

	errChan := make(chan error)
	go sockets.ListenAndWaitOnSocket(errChan, listener)
	return func() error {
		return <-errChan
	}, listener, nil
}
