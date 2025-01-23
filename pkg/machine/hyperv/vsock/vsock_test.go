//go:build windows

package vsock

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/containers/podman/v5/pkg/machine/windows"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/windows/registry"
)

func TestCreateHVSockRegistryEntry(t *testing.T) {
	originalFindOpenHVSockPort := hfindOpenHVSockPort
	defer func() { hfindOpenHVSockPort = originalFindOpenHVSockPort }()

	tests := []struct {
		name                   string
		machineName            string
		purpose                HVSockPurpose
		wantPort               uint64
		wantErr                bool
		findOpenHVSockPortMock func() (uint64, error)
	}{
		{
			name:        "ValidInput",
			machineName: "test-machine",
			purpose:     Network,
			wantPort:    1111,
			wantErr:     false,
			findOpenHVSockPortMock: func() (uint64, error) {
				return 1111, nil
			},
		},
		{
			name:        "ErrorFindPort",
			machineName: "test-machine",
			purpose:     Events,
			wantErr:     true,
			findOpenHVSockPortMock: func() (uint64, error) {
				return 0, errors.New("error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hfindOpenHVSockPort = tt.findOpenHVSockPortMock
			got, err := CreateHVSockRegistryEntry(tt.machineName, tt.purpose)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantPort, got.Port)
				assert.Equal(t, tt.machineName, got.MachineName)
				assert.Equal(t, tt.purpose, got.Purpose)
			}
		})
	}
}

func TestElevateAndAddEntries(t *testing.T) {
	resultTestScript := make(map[string]string)

	tests := []struct {
		name                                    string
		entries                                 []HVSockRegistryEntry
		err                                     error
		wantErr                                 bool
		registryOpenKeyMock                     func(k registry.Key, path string, access uint32) (registry.Key, error)
		windowsLaunchElevatedWaitWithWindowMode func(exe string, cwd string, args string, windowMode int) error
		script                                  string
	}{
		{
			name: "ErrorInvalidPort",
			entries: []HVSockRegistryEntry{
				{
					KeyName:     "key",
					Purpose:     Events,
					Port:        0,
					MachineName: "test-machine",
				},
			},
			err:     errors.New("port must be larger than 1"),
			wantErr: true,
		},
		{
			name: "ErrorInvalidPurpose",
			entries: []HVSockRegistryEntry{
				{
					KeyName:     "key",
					Purpose:     HVSockPurpose(999),
					Port:        8888,
					MachineName: "test-machine",
				},
			},
			err:     errors.New("required field purpose is empty"),
			wantErr: true,
		},
		{
			name: "ErrorEmptyMachineName",
			entries: []HVSockRegistryEntry{
				{
					KeyName:     "key",
					Purpose:     Events,
					Port:        8888,
					MachineName: "",
				},
			},
			err:     errors.New("required field machinename is empty"),
			wantErr: true,
		},
		{
			name: "ErrorEmptyKeyName",
			entries: []HVSockRegistryEntry{
				{
					KeyName:     "",
					Purpose:     Events,
					Port:        8888,
					MachineName: "test-machine",
				},
			},
			err:     errors.New("required field keypath is empty"),
			wantErr: true,
		},
		{
			name: "ErrorCannotOpenRegistryEntry",
			entries: []HVSockRegistryEntry{
				{
					KeyName:     "key",
					Purpose:     Events,
					Port:        8888,
					MachineName: "test-machine",
				},
			},
			err:     errors.New("cannot open my mocked registry key"),
			wantErr: true,
			registryOpenKeyMock: func(k registry.Key, path string, access uint32) (registry.Key, error) {
				return k, errors.New("cannot open my mocked registry key")
			},
		},
		{
			name: "ErrorRegistryEntryExists",
			entries: []HVSockRegistryEntry{
				{
					KeyName:     "key",
					Purpose:     Events,
					Port:        8888,
					MachineName: "test-machine",
				},
			},
			err:     fmt.Errorf("%q: %s", ErrVSockRegistryEntryExists, "key"),
			wantErr: true,
			registryOpenKeyMock: func(k registry.Key, path string, access uint32) (registry.Key, error) {
				return k, nil
			},
		},
		{
			name: "ErrorRegistryEntryExists",
			entries: []HVSockRegistryEntry{
				{
					KeyName:     "key",
					Purpose:     Events,
					Port:        8888,
					MachineName: "test-machine",
				},
			},
			err:     errors.New("error when opening parent key"),
			wantErr: true,
			registryOpenKeyMock: func(k registry.Key, path string, access uint32) (registry.Key, error) {
				if path == VsockRegistryPath {
					return k, errors.New("error when opening parent key")
				}
				return k, registry.ErrNotExist
			},
		},
		{
			name: "ValidInput",
			entries: []HVSockRegistryEntry{
				{
					KeyName:     "key",
					Purpose:     Events,
					Port:        8888,
					MachineName: "test-machine",
				},
			},
			wantErr: false,
			registryOpenKeyMock: func(k registry.Key, path string, access uint32) (registry.Key, error) {
				if path == VsockRegistryPath {
					return k, nil
				}
				return k, registry.ErrNotExist
			},
			windowsLaunchElevatedWaitWithWindowMode: func(exe, cwd, args string, windowMode int) error {
				resultTestScript["ValidInput"] = args
				return nil
			},
			script: "New-Item -Path 'HKLM:\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\\Virtualization\\GuestCommunicationServices' -Name 'key'; New-ItemProperty -Path 'HKLM:\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\\Virtualization\\GuestCommunicationServices\\key' -Name 'Purpose' -Value 'Events' -PropertyType String; New-ItemProperty -Path 'HKLM:\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\\Virtualization\\GuestCommunicationServices\\key' -Name 'MachineName' -Value 'test-machine' -PropertyType String;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.registryOpenKeyMock != nil {
				originalegistryOpenKey := rOpenKey
				defer func() { rOpenKey = originalegistryOpenKey }()
				rOpenKey = tt.registryOpenKeyMock
			}
			if tt.windowsLaunchElevatedWaitWithWindowMode != nil {
				originalLaunchElevatedWaitWithWindowMode := wLaunchElevatedWaitWithWindowMode
				defer func() { wLaunchElevatedWaitWithWindowMode = originalLaunchElevatedWaitWithWindowMode }()
				wLaunchElevatedWaitWithWindowMode = tt.windowsLaunchElevatedWaitWithWindowMode
			}
			err := ElevateAndAddEntries(tt.entries)

			if tt.wantErr {
				assert.Error(t, err)
				assert.EqualValues(t, err, tt.err)
			} else {
				assert.NoError(t, err)
				resultingScript := resultTestScript[tt.name]
				assert.EqualValues(t, resultingScript, tt.script)
			}
		})
	}
}

func TestElevateAndRemoveEntries(t *testing.T) {
	expectedScript := "Remove-Item -Path 'HKLM:\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\\Virtualization\\GuestCommunicationServices\\key' -Force -Recurse;"
	script := ""
	originalLaunchElevatedWaitWithWindowMode := wLaunchElevatedWaitWithWindowMode
	defer func() { wLaunchElevatedWaitWithWindowMode = originalLaunchElevatedWaitWithWindowMode }()
	wLaunchElevatedWaitWithWindowMode = func(exe, cwd, args string, windowMode int) error {
		script = args
		return nil
	}

	err := ElevateAndRemoveEntries([]HVSockRegistryEntry{
		{
			KeyName:     "key",
			Purpose:     Events,
			Port:        8888,
			MachineName: "test-machine",
		},
	})
	assert.NoError(t, err)
	assert.EqualValues(t, script, expectedScript)
}

func TestVerifyPowershellScriptActuallyWork(t *testing.T) {
	// this test is used to verify that the powershell script works as expected and the registry is actually written.
	// To avoid requiring admin privileges the original path
	// HKLM:\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\\Virtualization\\GuestCommunicationServices\\key
	// gets replaced by
	// HKCU:\\Software

	// the openKey func gets called after the path have been edited
	originalegistryOpenKey := rOpenKey
	defer func() { rOpenKey = originalegistryOpenKey }()
	rOpenKey = func(k registry.Key, path string, access uint32) (registry.Key, error) {
		path = strings.ReplaceAll(path, "SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\\Virtualization\\GuestCommunicationServices", "Software")
		return registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
	}

	// the LaunchElevatedWaitWithWindowMode gets replaced by its "no-elevated" version that executes the script as a normal user
	originalLaunchElevatedWaitWithWindowMode := wLaunchElevatedWaitWithWindowMode
	defer func() { wLaunchElevatedWaitWithWindowMode = originalLaunchElevatedWaitWithWindowMode }()
	wLaunchElevatedWaitWithWindowMode = func(exe, cwd, args string, windowMode int) error {
		args = strings.ReplaceAll(args, "HKLM:\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\\Virtualization\\GuestCommunicationServices", "HKCU:\\Software")
		return windows.LaunchWaitWithWindowMode(exe, cwd, args, windowMode)
	}

	entries := []HVSockRegistryEntry{
		{
			KeyName:     "key",
			Purpose:     Events,
			Port:        8888,
			MachineName: "test-machine",
		},
		{
			KeyName:     "key1",
			Purpose:     Fileserver,
			Port:        8889,
			MachineName: "test-machine",
		},
		{
			KeyName:     "key2",
			Purpose:     Network,
			Port:        8890,
			MachineName: "test-machine",
		},
	}

	// call the func to add entries to the registry
	err := ElevateAndAddEntries(entries)
	assert.NoError(t, err)

	// check the registry entries have been created
	for _, entry := range entries {
		_, err = registry.OpenKey(registry.CURRENT_USER, fmt.Sprintf("%s\\%s", "Software", entry.KeyName), registry.QUERY_VALUE)
		assert.NoError(t, err)
	}

	// remove entries from registry
	err = ElevateAndRemoveEntries(entries)
	assert.NoError(t, err)

	// check the registry entries do not exist
	for _, entry := range entries {
		_, err = registry.OpenKey(registry.CURRENT_USER, fmt.Sprintf("%s\\%s", "Software", entry.KeyName), registry.QUERY_VALUE)
		assert.ErrorIs(t, err, registry.ErrNotExist)
	}
}
