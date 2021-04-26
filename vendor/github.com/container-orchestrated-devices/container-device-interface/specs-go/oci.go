package specs

import (
	"errors"
	"fmt"

	spec "github.com/opencontainers/runtime-spec/specs-go"
)

// ApplyOCIEditsForDevice applies devices OCI edits, in other words
// it finds the device in the CDI spec and applies the OCI patches that device
// requires to the OCI specification.
func ApplyOCIEditsForDevice(config *spec.Spec, cdi *Spec, dev string) error {
	for _, d := range cdi.Devices {
		if d.Name != dev {
			continue
		}

		return ApplyEditsToOCISpec(config, &d.ContainerEdits)
	}

	return fmt.Errorf("CDI: device %q not found for spec %q", dev, cdi.Kind)
}

// ApplyOCIEdits applies the OCI edits the CDI spec declares globablly
func ApplyOCIEdits(config *spec.Spec, cdi *Spec) error {
	return ApplyEditsToOCISpec(config, &cdi.ContainerEdits)
}

// ApplyEditsToOCISpec applies the specified edits to the OCI spec.
func ApplyEditsToOCISpec(config *spec.Spec, edits *ContainerEdits) error {
	if config == nil {
		return errors.New("spec is nil")
	}
	if edits == nil {
		return nil
	}

	if len(edits.Env) > 0 {

		if config.Process == nil {
			config.Process = &spec.Process{}
		}

		config.Process.Env = append(config.Process.Env, edits.Env...)
	}

	for _, d := range edits.DeviceNodes {
		config.Mounts = append(config.Mounts, toOCIDevice(d))
	}

	for _, m := range edits.Mounts {
		config.Mounts = append(config.Mounts, toOCIMount(m))
	}

	for _, h := range edits.Hooks {
		if config.Hooks == nil {
			config.Hooks = &spec.Hooks{}
		}
		switch h.HookName {
		case "prestart":
			config.Hooks.Prestart = append(config.Hooks.Prestart, toOCIHook(h))
		case "createRuntime":
			config.Hooks.CreateRuntime = append(config.Hooks.CreateRuntime, toOCIHook(h))
		case "createContainer":
			config.Hooks.CreateContainer = append(config.Hooks.CreateContainer, toOCIHook(h))
		case "startContainer":
			config.Hooks.StartContainer = append(config.Hooks.StartContainer, toOCIHook(h))
		case "poststart":
			config.Hooks.Poststart = append(config.Hooks.Poststart, toOCIHook(h))
		case "poststop":
			config.Hooks.Poststop = append(config.Hooks.Poststop, toOCIHook(h))
		default:
			fmt.Printf("CDI: Unknown hook %q\n", h.HookName)
		}
	}

	return nil
}

func toOCIHook(h *Hook) spec.Hook {
	return spec.Hook{
		Path:    h.Path,
		Args:    h.Args,
		Env:     h.Env,
		Timeout: h.Timeout,
	}
}

func toOCIMount(m *Mount) spec.Mount {
	return spec.Mount{
		Source:      m.HostPath,
		Destination: m.ContainerPath,
		Options:     m.Options,
	}
}

func toOCIDevice(d *DeviceNode) spec.Mount {
	return spec.Mount{
		Source:      d.HostPath,
		Destination: d.ContainerPath,
		Options:     d.Permissions,
	}
}
