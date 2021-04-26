package pkg

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	cdispec "github.com/container-orchestrated-devices/container-device-interface/specs-go"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

const (
	root = "/etc/cdi"
)

func collectCDISpecs() (map[string]*cdispec.Spec, error) {
	var files []string
	vendor := make(map[string]*cdispec.Spec)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if info == nil || info.IsDir() {
			return nil
		}

		if filepath.Ext(path) != ".json" {
			return nil
		}

		files = append(files, path)
		return nil
	})

	if err != nil {
		return nil, err
	}

	for _, path := range files {
		spec, err := loadCDIFile(path)
		if err != nil {
			continue
		}

		if _, ok := vendor[spec.Kind]; ok {
			continue
		}

		vendor[spec.Kind] = spec
	}

	return vendor, nil
}

// TODO: Validate (e.g: duplicate device names)
func loadCDIFile(path string) (*cdispec.Spec, error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var spec *cdispec.Spec
	err = json.Unmarshal([]byte(file), &spec)
	if err != nil {
		return nil, err
	}

	return spec, nil
}

/*
* Pattern "vendor.com/device=myDevice" with the vendor being optional
 */
func extractVendor(dev string) (string, string) {
	if strings.IndexByte(dev, '=') == -1 {
		return "", dev
	}

	split := strings.SplitN(dev, "=", 2)
	return split[0], split[1]
}

// GetCDIForDevice returns the CDI specification that matches the device name the user provided.
func GetCDIForDevice(dev string, specs map[string]*cdispec.Spec) (*cdispec.Spec, error) {
	vendor, device := extractVendor(dev)

	if vendor != "" {
		s, ok := specs[vendor]
		if !ok {
			return nil, fmt.Errorf("Could not find vendor %q for device %q", vendor, device)
		}

		for _, d := range s.Devices {
			if d.Name != device {
				continue
			}

			return s, nil
		}

		return nil, fmt.Errorf("Could not find device %q for vendor %q", device, vendor)
	}

	var found []*cdispec.Spec
	var vendors []string
	for vendor, spec := range specs {

		for _, d := range spec.Devices {
			if d.Name != device {
				continue
			}

			found = append(found, spec)
			vendors = append(vendors, vendor)
		}
	}

	if len(found) > 1 {
		return nil, fmt.Errorf("%q is ambiguous and currently refers to multiple devices from different vendors: %q", dev, vendors)
	}

	if len(found) == 1 {
		return found[0], nil
	}

	return nil, fmt.Errorf("Could not find device %q", dev)
}

// HasDevice returns true if a device is a CDI device
// an error may be returned in cases where permissions may be required
func HasDevice(dev string) (bool, error) {
	specs, err := collectCDISpecs()
	if err != nil {
		return false, err
	}

	d, err := GetCDIForDevice(dev, specs)
	if err != nil {
		return false, err
	}

	return d != nil, nil
}

// UpdateOCISpecForDevices updates the given OCI spec based on the requested CDI devices
func UpdateOCISpecForDevices(ociconfig *spec.Spec, devs []string) error {
	specs, err := collectCDISpecs()
	if err != nil {
		return err
	}

	return UpdateOCISpecForDevicesWithSpec(ociconfig, devs, specs)
}

// UpdateOCISpecForDevicesWithLoggerAndSpecs is mainly used for testing
func UpdateOCISpecForDevicesWithSpec(ociconfig *spec.Spec, devs []string, specs map[string]*cdispec.Spec) error {
	edits := make(map[string]*cdispec.Spec)

	for _, d := range devs {
		spec, err := GetCDIForDevice(d, specs)
		if err != nil {
			return err
		}

		edits[spec.Kind] = spec
		err = cdispec.ApplyOCIEditsForDevice(ociconfig, spec, d)
		if err != nil {
			return err
		}
	}

	for _, spec := range edits {
		if err := cdispec.ApplyOCIEdits(ociconfig, spec); err != nil {
			return err
		}
	}

	return nil
}
