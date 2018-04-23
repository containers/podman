package varlinkapi

import (
	ioprojectatomicpodman "github.com/projectatomic/libpod/cmd/podman/varlink"
)

// ListContainers ...
func (i *LibpodAPI) ListContainers(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("ListContainers")
}

// CreateContainer ...
func (i *LibpodAPI) CreateContainer(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("CreateContainer")
}

// InspectContainer ...
func (i *LibpodAPI) InspectContainer(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("InspectContainer")
}

// ListContainerProcesses ...
func (i *LibpodAPI) ListContainerProcesses(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("ListContainerProcesses")
}

// GetContainerLogs ...
func (i *LibpodAPI) GetContainerLogs(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("GetContainerLogs")
}

// ListContainerChanges ...
func (i *LibpodAPI) ListContainerChanges(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("ListContianerChanges")
}

// ExportContainer ...
func (i *LibpodAPI) ExportContainer(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("ExportContainer")
}

// GetContainerStats ...
func (i *LibpodAPI) GetContainerStats(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("GetContainerStates")
}

// ResizeContainerTty ...
func (i *LibpodAPI) ResizeContainerTty(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("ResizeContainerTty")
}

// StartContainer ...
func (i *LibpodAPI) StartContainer(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("StartContainer")
}

// StopContainer ...
func (i *LibpodAPI) StopContainer(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("StopContainer")
}

// RestartContainer ...
func (i *LibpodAPI) RestartContainer(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("RestartContainer")
}

// KillContainer ...
func (i *LibpodAPI) KillContainer(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("KillContainer")
}

// UpdateContainer ...
func (i *LibpodAPI) UpdateContainer(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("UpdateContainer")
}

// RenameContainer ...
func (i *LibpodAPI) RenameContainer(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("RenameContainer")
}

// PauseContainer ...
func (i *LibpodAPI) PauseContainer(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("PauseContainer")
}

// UnpauseContainer ...
func (i *LibpodAPI) UnpauseContainer(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("UnpauseContainer")
}

// AttachToContainer ...
// TODO: DO we also want a different one for websocket?
func (i *LibpodAPI) AttachToContainer(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("AttachToContainer")
}

// WaitContainer ...
func (i *LibpodAPI) WaitContainer(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("WaitContainer")
}

// RemoveContainer ...
func (i *LibpodAPI) RemoveContainer(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("RemoveContainer")
}

// DeleteStoppedContainers ...
func (i *LibpodAPI) DeleteStoppedContainers(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("DeleteContainer")
}
