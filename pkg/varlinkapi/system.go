package varlinkapi

import (
	"github.com/projectatomic/libpod/cmd/podman/ioprojectatomicpodman"
	"github.com/projectatomic/libpod/libpod"
)

// GetVersion ...
func (i *LibpodAPI) GetVersion(call ioprojectatomicpodman.VarlinkCall) error {
	versionInfo, err := libpod.GetVersion()
	if err != nil {
		return err
	}

	return call.ReplyGetVersion(ioprojectatomicpodman.Version{
		Version:    versionInfo.Version,
		Go_version: versionInfo.GoVersion,
		Git_commit: versionInfo.GitCommit,
		Built:      versionInfo.Built,
		Os_arch:    versionInfo.OsArch,
	})
}

// Ping returns a simple string "OK" response for clients to make sure
// the service is working.
func (i *LibpodAPI) Ping(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyPing(ioprojectatomicpodman.StringResponse{
		Message: "OK",
	})
}
