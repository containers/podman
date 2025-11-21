//go:build windows

package vsock

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/Microsoft/go-winio"
	"github.com/containers/podman/v6/pkg/machine/sockets"
	"github.com/containers/podman/v6/utils"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows/registry"
)

var ErrVSockRegistryEntryExists = errors.New("registry entry already exists")

const (
	// HvsockMachineName is the string identifier for the machine name in a registry entry
	HvsockMachineName = "MachineName"
	// HvsockToolName is the string identifier for the tool name in a registry entry
	HvsockToolName = "ToolName"
	PodmanToolName = "podman"
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
	KeyName string        `json:"key_name"`
	Purpose HVSockPurpose `json:"purpose"`
	Port    uint64        `json:"port"`

	// MachineName is deprecated.
	// Registry entries are now shared across machines, so a machine-specific identifier isn't appropriate here.
	MachineName string `json:"machineName,omitempty"`

	// ToolName identifies the application that created this registry entry (e.g., Podman).
	// This provides information about the entry's origin and can be used for filter entries
	// if purpose is not enough.
	ToolName string       `json:"creator_tool,omitempty"`
	Key      registry.Key `json:"key,omitempty"`
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
	return newKey.SetStringValue(HvsockToolName, hv.ToolName)
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
	if len(hv.ToolName) < 1 {
		return errors.New("required field toolName is empty")
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
func NewHVSockRegistryEntry(purpose HVSockPurpose) (*HVSockRegistryEntry, error) {
	// a so-called wildcard entry ... everything from FACB -> 6D3 is MS special sauce
	// for a " linux vm".  this first segment is hexi for the hvsock port number
	// 00000400-FACB-11E6-BD58-64006A7986D3
	port, err := findOpenHVSockPort()
	if err != nil {
		return nil, err
	}
	r := HVSockRegistryEntry{
		KeyName:  portToKeyName(port),
		Purpose:  purpose,
		Port:     port,
		ToolName: PodmanToolName,
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

	m, _, err := k.GetStringValue(HvsockMachineName)
	if err != nil {
		return nil, err
	}

	return &HVSockRegistryEntry{
		KeyName:     keyName,
		Purpose:     purpose,
		Port:        port,
		Key:         k,
		MachineName: m,
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

// loadAllHVSockRegistryEntries loads HVSock registry entries, filtered by purpose and optionally limited by size.
// If limit is -1, it returns all matching entries. Otherwise, it returns up to 'limit' entries.
// The caller is responsible for closing the registry.Key in each returned HVSockRegistryEntry.
// Non-matching or excess keys are closed within this function.
func loadHVSockRegistryEntries(purpose HVSockPurpose, limit int) ([]*HVSockRegistryEntry, error) {
	parentKey, err := registry.OpenKey(registry.LOCAL_MACHINE, VsockRegistryPath, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		logrus.Errorf("failed to open registry key: %s: %v", VsockRegistryPath, err)
		return nil, err
	}
	defer func() {
		if err := parentKey.Close(); err != nil {
			logrus.Errorf("failed to close registry key: %v", err)
		}
	}()

	subKeyNames, err := parentKey.ReadSubKeyNames(-1)
	if err != nil {
		logrus.Errorf("failed to read subkey names from %s: %v", VsockRegistryPath, err)
		return nil, err
	}

	allEntries := []*HVSockRegistryEntry{}
	for _, subKeyName := range subKeyNames {
		if limit != -1 && len(allEntries) >= limit {
			break
		}

		fqPath := fmt.Sprintf("%s\\%s", VsockRegistryPath, subKeyName)
		k, err := openVSockRegistryEntry(fqPath)
		if err != nil {
			logrus.Debugf("Could not open registry entry %s: %v", fqPath, err)
			continue
		}

		p, _, err := k.GetStringValue(HvsockPurpose)
		if err != nil {
			logrus.Debugf("Could not read purpose from registry entry %s: %v", fqPath, err)
			k.Close()
			continue
		}

		toolName, _, err := k.GetStringValue(HvsockToolName)
		if err != nil {
			logrus.Debugf("Could not read tool name from registry entry %s: %v", fqPath, err)
			k.Close()
			continue
		}

		k.Close()

		entryPurpose, err := toHVSockPurpose(p)
		if err != nil {
			logrus.Debugf("Could not convert purpose string %q for entry %s: %v", p, fqPath, err)
			continue
		}

		if !entryPurpose.Equal(purpose.string()) {
			continue
		}

		if toolName != PodmanToolName {
			continue
		}

		parts := strings.Split(subKeyName, "-")
		if len(parts) == 0 {
			logrus.Debugf("Malformed key name %s: cannot extract port", subKeyName)
			continue
		}

		portHex := parts[0]
		port, err := parseHexToUint64(portHex)
		if err != nil {
			logrus.Debugf("Could not parse port from key name %s: %v", subKeyName, err)
			continue
		}

		allEntries = append(allEntries, &HVSockRegistryEntry{
			KeyName:  subKeyName,
			Purpose:  entryPurpose,
			Port:     port,
			Key:      k,
			ToolName: PodmanToolName,
		})
	}

	return allEntries, nil
}

func LoadHVSockRegistryEntryByPurpose(purpose HVSockPurpose) (*HVSockRegistryEntry, error) {
	entries, err := loadHVSockRegistryEntries(purpose, 1)
	if err != nil {
		return nil, err
	}
	if len(entries) != 1 {
		return nil, fmt.Errorf("no hvsock registry entry found for purpose: %s", purpose.string())
	}

	return entries[0], nil
}

func LoadAllHVSockRegistryEntriesByPurpose(purpose HVSockPurpose) ([]*HVSockRegistryEntry, error) {
	entries, err := loadHVSockRegistryEntries(purpose, -1)
	if err != nil {
		return nil, err
	}

	return entries, nil
}

func parseHexToUint64(hex string) (uint64, error) {
	return strconv.ParseUint(hex, 16, 64)
}

// It removes HVSock registry entries for Network, Events, and Fileserver.
// It returns loading errors immediately. For removals, it attempts all, logs individual failures,
// and returns a joined error (via errors.Join) if any occur.
// Returns nil only if all entries are loaded and removed successfully.
func RemoveAllHVSockRegistryEntries() error {
	// Tear down vsocks
	networkSocks, err := LoadAllHVSockRegistryEntriesByPurpose(Network)
	if err != nil {
		return err
	}
	eventsSocks, err := LoadAllHVSockRegistryEntriesByPurpose(Events)
	if err != nil {
		return err
	}
	fileserverSocks, err := LoadAllHVSockRegistryEntriesByPurpose(Fileserver)
	if err != nil {
		return err
	}

	allSocks := []*HVSockRegistryEntry{}
	allSocks = append(allSocks, networkSocks...)
	allSocks = append(allSocks, eventsSocks...)
	allSocks = append(allSocks, fileserverSocks...)

	var removalErrors []error
	for _, sock := range allSocks {
		if err := sock.Remove(); err != nil {
			logrus.Errorf("unable to remove registry entry for %s: %q", sock.KeyName, err)
			removalErrors = append(removalErrors, fmt.Errorf("failed to remove sock %s: %w", sock.KeyName, err))
		}
	}

	if len(removalErrors) > 0 {
		return errors.Join(removalErrors...)
	}

	return nil
}
