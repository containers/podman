/*
   Copyright © 2021 The CDI Authors

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package cdi

import (
	"strings"

	"github.com/pkg/errors"

	"github.com/container-orchestrated-devices/container-device-interface/specs-go"
	oci "github.com/opencontainers/runtime-spec/specs-go"
	ocigen "github.com/opencontainers/runtime-tools/generate"
)

const (
	// PrestartHook is the name of the OCI "prestart" hook.
	PrestartHook = "prestart"
	// CreateRuntimeHook is the name of the OCI "createRuntime" hook.
	CreateRuntimeHook = "createRuntime"
	// CreateContainerHook is the name of the OCI "createContainer" hook.
	CreateContainerHook = "createContainer"
	// StartContainerHook is the name of the OCI "startContainer" hook.
	StartContainerHook = "startContainer"
	// PoststartHook is the name of the OCI "poststart" hook.
	PoststartHook = "poststart"
	// PoststopHook is the name of the OCI "poststop" hook.
	PoststopHook = "poststop"
)

var (
	// Names of recognized hooks.
	validHookNames = map[string]struct{}{
		PrestartHook:        {},
		CreateRuntimeHook:   {},
		CreateContainerHook: {},
		StartContainerHook:  {},
		PoststartHook:       {},
		PoststopHook:        {},
	}
)

// ContainerEdits represent updates to be applied to an OCI Spec.
// These updates can be specific to a CDI device, or they can be
// specific to a CDI Spec. In the former case these edits should
// be applied to all OCI Specs where the corresponding CDI device
// is injected. In the latter case, these edits should be applied
// to all OCI Specs where at least one devices from the CDI Spec
// is injected.
type ContainerEdits struct {
	*specs.ContainerEdits
}

// Apply edits to the given OCI Spec. Updates the OCI Spec in place.
// Returns an error if the update fails.
func (e *ContainerEdits) Apply(spec *oci.Spec) error {
	if spec == nil {
		return errors.New("can't edit nil OCI Spec")
	}
	if e == nil || e.ContainerEdits == nil {
		return nil
	}

	specgen := ocigen.NewFromSpec(spec)
	if len(e.Env) > 0 {
		specgen.AddMultipleProcessEnv(e.Env)
	}
	for _, d := range e.DeviceNodes {
		specgen.AddDevice(d.ToOCI())
	}
	for _, m := range e.Mounts {
		specgen.AddMount(m.ToOCI())
	}
	for _, h := range e.Hooks {
		switch h.HookName {
		case PrestartHook:
			specgen.AddPreStartHook(h.ToOCI())
		case PoststartHook:
			specgen.AddPostStartHook(h.ToOCI())
		case PoststopHook:
			specgen.AddPostStopHook(h.ToOCI())
			// TODO: Maybe runtime-tools/generate should be updated with these...
		case CreateRuntimeHook:
			ensureOCIHooks(spec)
			spec.Hooks.CreateRuntime = append(spec.Hooks.CreateRuntime, h.ToOCI())
		case CreateContainerHook:
			ensureOCIHooks(spec)
			spec.Hooks.CreateContainer = append(spec.Hooks.CreateContainer, h.ToOCI())
		case StartContainerHook:
			ensureOCIHooks(spec)
			spec.Hooks.StartContainer = append(spec.Hooks.StartContainer, h.ToOCI())
		default:
			return errors.Errorf("unknown hook name %q", h.HookName)
		}
	}

	return nil
}

// Validate container edits.
func (e *ContainerEdits) Validate() error {
	if e == nil || e.ContainerEdits == nil {
		return nil
	}

	if err := ValidateEnv(e.Env); err != nil {
		return errors.Wrap(err, "invalid container edits")
	}
	for _, d := range e.DeviceNodes {
		if err := (&DeviceNode{d}).Validate(); err != nil {
			return err
		}
	}
	for _, h := range e.Hooks {
		if err := (&Hook{h}).Validate(); err != nil {
			return err
		}
	}
	for _, m := range e.Mounts {
		if err := (&Mount{m}).Validate(); err != nil {
			return err
		}
	}

	return nil
}

// isEmpty returns true if these edits are empty. This is valid in a
// global Spec context but invalid in a Device context.
func (e *ContainerEdits) isEmpty() bool {
	if e == nil {
		return false
	}
	return len(e.Env)+len(e.DeviceNodes)+len(e.Hooks)+len(e.Mounts) == 0
}

// ValidateEnv validates the given environment variables.
func ValidateEnv(env []string) error {
	for _, v := range env {
		if strings.IndexByte(v, byte('=')) <= 0 {
			return errors.Errorf("invalid environment variable %q", v)
		}
	}
	return nil
}

// DeviceNode is a CDI Spec DeviceNode wrapper, used for validating DeviceNodes.
type DeviceNode struct {
	*specs.DeviceNode
}

// Validate a CDI Spec DeviceNode.
func (d *DeviceNode) Validate() error {
	if d.Path == "" {
		return errors.New("invalid (empty) device path")
	}
	if d.Type != "" && d.Type != "b" && d.Type != "c" {
		return errors.Errorf("device %q: invalid type %q", d.Path, d.Type)
	}
	for _, bit := range d.Permissions {
		if bit != 'r' && bit != 'w' && bit != 'm' {
			return errors.Errorf("device %q: invalid persmissions %q",
				d.Path, d.Permissions)
		}
	}
	return nil
}

// Hook is a CDI Spec Hook wrapper, used for validating hooks.
type Hook struct {
	*specs.Hook
}

// Validate a hook.
func (h *Hook) Validate() error {
	if _, ok := validHookNames[h.HookName]; !ok {
		return errors.Errorf("invalid hook name %q", h.HookName)
	}
	if h.Path == "" {
		return errors.Errorf("invalid hook %q with empty path", h.HookName)
	}
	if err := ValidateEnv(h.Env); err != nil {
		return errors.Wrapf(err, "invalid hook %q", h.HookName)
	}
	return nil
}

// Mount is a CDI Mount wrapper, used for validating mounts.
type Mount struct {
	*specs.Mount
}

// Validate a mount.
func (m *Mount) Validate() error {
	if m.HostPath == "" {
		return errors.New("invalid mount, empty host path")
	}
	if m.ContainerPath == "" {
		return errors.New("invalid mount, empty container path")
	}
	return nil
}

// Ensure OCI Spec hooks are not nil so we can add hooks.
func ensureOCIHooks(spec *oci.Spec) {
	if spec.Hooks == nil {
		spec.Hooks = &oci.Hooks{}
	}
}
