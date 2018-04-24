// Generated with github.com/varlink/go/cmd/varlink-go-interface-generator
package ioprojectatomicpodman

import "github.com/varlink/go/varlink"

type Version struct{
	Version string `json:"version"`
	Go_version string `json:"go_version"`
	Git_commit string `json:"git_commit"`
	Built int64 `json:"built"`
	Os_arch string `json:"os_arch"`
}

type NotImplemented struct{
	Comment string `json:"comment"`
}

type StringResponse struct{
	Message string `json:"message"`
}

type ioprojectatomicpodmanInterface interface {
	ListContainerProcesses(c VarlinkCall) error
	UpdateContainer(c VarlinkCall) error
	HistoryImage(c VarlinkCall) error
	ExportImage(c VarlinkCall) error
	CreateContainer(c VarlinkCall) error
	InspectContainer(c VarlinkCall) error
	UnpauseContainer(c VarlinkCall) error
	AttachToContainer(c VarlinkCall) error
	SearchImage(c VarlinkCall) error
	ListContainerChanges(c VarlinkCall) error
	RenameContainer(c VarlinkCall) error
	RemoveContainer(c VarlinkCall) error
	PullImage(c VarlinkCall) error
	Ping(c VarlinkCall) error
	ListContainers(c VarlinkCall) error
	PushImage(c VarlinkCall) error
	TagImage(c VarlinkCall) error
	RemoveImage(c VarlinkCall) error
	KillContainer(c VarlinkCall) error
	InspectImage(c VarlinkCall) error
	CreateImage(c VarlinkCall) error
	DeleteUnusedImages(c VarlinkCall) error
	CreateFromContainer(c VarlinkCall) error
	ImportImage(c VarlinkCall) error
	PauseContainer(c VarlinkCall) error
	ListImages(c VarlinkCall) error
	DeleteStoppedContainers(c VarlinkCall) error
	ExportContainer(c VarlinkCall) error
	StartContainer(c VarlinkCall) error
	ResizeContainerTty(c VarlinkCall) error
	RestartContainer(c VarlinkCall) error
	WaitContainer(c VarlinkCall) error
	GetVersion(c VarlinkCall) error
	GetContainerStats(c VarlinkCall) error
	BuildImage(c VarlinkCall) error
	GetContainerLogs(c VarlinkCall) error
	StopContainer(c VarlinkCall) error
}

type VarlinkCall struct{ varlink.Call }

func (c *VarlinkCall) ReplyActionFailed(reason string) error {
	var out struct{
		Reason string `json:"reason"`
	}
	out.Reason = reason
	return c.ReplyError("io.projectatomic.podman.ActionFailed", &out)
}

func (c *VarlinkCall) ReplyGetVersion(version Version) error {
	var out struct{
		Version Version `json:"version"`
	}
	out.Version = version
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyGetContainerStats(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyResizeContainerTty(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyRestartContainer(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyWaitContainer(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyGetContainerLogs(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyStopContainer(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyBuildImage(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyCreateContainer(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyInspectContainer(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyListContainerProcesses(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyUpdateContainer(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyHistoryImage(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyExportImage(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyListContainerChanges(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyRenameContainer(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyUnpauseContainer(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyAttachToContainer(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplySearchImage(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyPing(ping StringResponse) error {
	var out struct{
		Ping StringResponse `json:"ping"`
	}
	out.Ping = ping
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyListContainers(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyRemoveContainer(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyPullImage(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyKillContainer(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyInspectImage(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyPushImage(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyTagImage(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyRemoveImage(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyPauseContainer(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyListImages(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyCreateImage(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyDeleteUnusedImages(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyCreateFromContainer(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyImportImage(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyExportContainer(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyStartContainer(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyDeleteStoppedContainers(notimplemented NotImplemented) error {
	var out struct{
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented
	return c.Reply(&out)
}

func (s *VarlinkInterface) DeleteUnusedImages(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("DeleteUnusedImages")
}

func (s *VarlinkInterface) CreateFromContainer(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("CreateFromContainer")
}

func (s *VarlinkInterface) ImportImage(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("ImportImage")
}

func (s *VarlinkInterface) PauseContainer(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("PauseContainer")
}

func (s *VarlinkInterface) ListImages(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("ListImages")
}

func (s *VarlinkInterface) CreateImage(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("CreateImage")
}

func (s *VarlinkInterface) ExportContainer(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("ExportContainer")
}

func (s *VarlinkInterface) StartContainer(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("StartContainer")
}

func (s *VarlinkInterface) DeleteStoppedContainers(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("DeleteStoppedContainers")
}

func (s *VarlinkInterface) RestartContainer(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("RestartContainer")
}

func (s *VarlinkInterface) WaitContainer(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("WaitContainer")
}

func (s *VarlinkInterface) GetVersion(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("GetVersion")
}

func (s *VarlinkInterface) GetContainerStats(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("GetContainerStats")
}

func (s *VarlinkInterface) ResizeContainerTty(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("ResizeContainerTty")
}

func (s *VarlinkInterface) GetContainerLogs(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("GetContainerLogs")
}

func (s *VarlinkInterface) StopContainer(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("StopContainer")
}

func (s *VarlinkInterface) BuildImage(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("BuildImage")
}

func (s *VarlinkInterface) UpdateContainer(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("UpdateContainer")
}

func (s *VarlinkInterface) HistoryImage(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("HistoryImage")
}

func (s *VarlinkInterface) ExportImage(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("ExportImage")
}

func (s *VarlinkInterface) CreateContainer(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("CreateContainer")
}

func (s *VarlinkInterface) InspectContainer(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("InspectContainer")
}

func (s *VarlinkInterface) ListContainerProcesses(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("ListContainerProcesses")
}

func (s *VarlinkInterface) AttachToContainer(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("AttachToContainer")
}

func (s *VarlinkInterface) SearchImage(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("SearchImage")
}

func (s *VarlinkInterface) ListContainerChanges(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("ListContainerChanges")
}

func (s *VarlinkInterface) RenameContainer(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("RenameContainer")
}

func (s *VarlinkInterface) UnpauseContainer(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("UnpauseContainer")
}

func (s *VarlinkInterface) PullImage(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("PullImage")
}

func (s *VarlinkInterface) Ping(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("Ping")
}

func (s *VarlinkInterface) ListContainers(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("ListContainers")
}

func (s *VarlinkInterface) RemoveContainer(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("RemoveContainer")
}

func (s *VarlinkInterface) TagImage(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("TagImage")
}

func (s *VarlinkInterface) RemoveImage(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("RemoveImage")
}

func (s *VarlinkInterface) KillContainer(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("KillContainer")
}

func (s *VarlinkInterface) InspectImage(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("InspectImage")
}

func (s *VarlinkInterface) PushImage(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("PushImage")
}

func (s *VarlinkInterface) VarlinkDispatch(call varlink.Call, methodname string) error {
	switch methodname {
	case "ExportContainer":
		return s.ioprojectatomicpodmanInterface.ExportContainer(VarlinkCall{call})

	case "StartContainer":
		return s.ioprojectatomicpodmanInterface.StartContainer(VarlinkCall{call})

	case "DeleteStoppedContainers":
		return s.ioprojectatomicpodmanInterface.DeleteStoppedContainers(VarlinkCall{call})

	case "GetVersion":
		return s.ioprojectatomicpodmanInterface.GetVersion(VarlinkCall{call})

	case "GetContainerStats":
		return s.ioprojectatomicpodmanInterface.GetContainerStats(VarlinkCall{call})

	case "ResizeContainerTty":
		return s.ioprojectatomicpodmanInterface.ResizeContainerTty(VarlinkCall{call})

	case "RestartContainer":
		return s.ioprojectatomicpodmanInterface.RestartContainer(VarlinkCall{call})

	case "WaitContainer":
		return s.ioprojectatomicpodmanInterface.WaitContainer(VarlinkCall{call})

	case "GetContainerLogs":
		return s.ioprojectatomicpodmanInterface.GetContainerLogs(VarlinkCall{call})

	case "StopContainer":
		return s.ioprojectatomicpodmanInterface.StopContainer(VarlinkCall{call})

	case "BuildImage":
		return s.ioprojectatomicpodmanInterface.BuildImage(VarlinkCall{call})

	case "CreateContainer":
		return s.ioprojectatomicpodmanInterface.CreateContainer(VarlinkCall{call})

	case "InspectContainer":
		return s.ioprojectatomicpodmanInterface.InspectContainer(VarlinkCall{call})

	case "ListContainerProcesses":
		return s.ioprojectatomicpodmanInterface.ListContainerProcesses(VarlinkCall{call})

	case "UpdateContainer":
		return s.ioprojectatomicpodmanInterface.UpdateContainer(VarlinkCall{call})

	case "HistoryImage":
		return s.ioprojectatomicpodmanInterface.HistoryImage(VarlinkCall{call})

	case "ExportImage":
		return s.ioprojectatomicpodmanInterface.ExportImage(VarlinkCall{call})

	case "ListContainerChanges":
		return s.ioprojectatomicpodmanInterface.ListContainerChanges(VarlinkCall{call})

	case "RenameContainer":
		return s.ioprojectatomicpodmanInterface.RenameContainer(VarlinkCall{call})

	case "UnpauseContainer":
		return s.ioprojectatomicpodmanInterface.UnpauseContainer(VarlinkCall{call})

	case "AttachToContainer":
		return s.ioprojectatomicpodmanInterface.AttachToContainer(VarlinkCall{call})

	case "SearchImage":
		return s.ioprojectatomicpodmanInterface.SearchImage(VarlinkCall{call})

	case "Ping":
		return s.ioprojectatomicpodmanInterface.Ping(VarlinkCall{call})

	case "ListContainers":
		return s.ioprojectatomicpodmanInterface.ListContainers(VarlinkCall{call})

	case "RemoveContainer":
		return s.ioprojectatomicpodmanInterface.RemoveContainer(VarlinkCall{call})

	case "PullImage":
		return s.ioprojectatomicpodmanInterface.PullImage(VarlinkCall{call})

	case "KillContainer":
		return s.ioprojectatomicpodmanInterface.KillContainer(VarlinkCall{call})

	case "InspectImage":
		return s.ioprojectatomicpodmanInterface.InspectImage(VarlinkCall{call})

	case "PushImage":
		return s.ioprojectatomicpodmanInterface.PushImage(VarlinkCall{call})

	case "TagImage":
		return s.ioprojectatomicpodmanInterface.TagImage(VarlinkCall{call})

	case "RemoveImage":
		return s.ioprojectatomicpodmanInterface.RemoveImage(VarlinkCall{call})

	case "PauseContainer":
		return s.ioprojectatomicpodmanInterface.PauseContainer(VarlinkCall{call})

	case "ListImages":
		return s.ioprojectatomicpodmanInterface.ListImages(VarlinkCall{call})

	case "CreateImage":
		return s.ioprojectatomicpodmanInterface.CreateImage(VarlinkCall{call})

	case "DeleteUnusedImages":
		return s.ioprojectatomicpodmanInterface.DeleteUnusedImages(VarlinkCall{call})

	case "CreateFromContainer":
		return s.ioprojectatomicpodmanInterface.CreateFromContainer(VarlinkCall{call})

	case "ImportImage":
		return s.ioprojectatomicpodmanInterface.ImportImage(VarlinkCall{call})

	default:
		return call.ReplyMethodNotFound(methodname)
	}
}
func (s *VarlinkInterface) VarlinkGetName() string {
	return `io.projectatomic.podman`
}

func (s *VarlinkInterface) VarlinkGetDescription() string {
	return `# Podman Service Interface
interface io.projectatomic.podman

type Version (
  version: string,
  go_version: string,
  git_commit: string,
  built: int,
  os_arch: string
)

type NotImplemented (
    comment: string
)

type StringResponse (
    message: string
)

# System
method Ping() -> (ping: StringResponse)
method GetVersion() -> (version: Version)

# Containers
method ListContainers() -> (notimplemented: NotImplemented)
method CreateContainer() -> (notimplemented: NotImplemented)
method InspectContainer() -> (notimplemented: NotImplemented)
method ListContainerProcesses() -> (notimplemented: NotImplemented)
method GetContainerLogs() -> (notimplemented: NotImplemented)
method ListContainerChanges() -> (notimplemented: NotImplemented)
method ExportContainer() -> (notimplemented: NotImplemented)
method GetContainerStats() -> (notimplemented: NotImplemented)
method ResizeContainerTty() -> (notimplemented: NotImplemented)
method StartContainer() -> (notimplemented: NotImplemented)
method StopContainer() -> (notimplemented: NotImplemented)
method RestartContainer() -> (notimplemented: NotImplemented)
method KillContainer() -> (notimplemented: NotImplemented)
method UpdateContainer() -> (notimplemented: NotImplemented)
method RenameContainer() -> (notimplemented: NotImplemented)
method PauseContainer() -> (notimplemented: NotImplemented)
method UnpauseContainer() -> (notimplemented: NotImplemented)
method AttachToContainer() -> (notimplemented: NotImplemented)
method WaitContainer() -> (notimplemented: NotImplemented)
method RemoveContainer() -> (notimplemented: NotImplemented)
method DeleteStoppedContainers() -> (notimplemented: NotImplemented)

# Images
method ListImages() -> (notimplemented: NotImplemented)
method BuildImage() -> (notimplemented: NotImplemented)
method CreateImage() -> (notimplemented: NotImplemented)
method InspectImage() -> (notimplemented: NotImplemented)
method HistoryImage() -> (notimplemented: NotImplemented)
method PushImage() -> (notimplemented: NotImplemented)
method TagImage() -> (notimplemented: NotImplemented)
method RemoveImage() -> (notimplemented: NotImplemented)
method SearchImage() -> (notimplemented: NotImplemented)
method DeleteUnusedImages() -> (notimplemented: NotImplemented)
method CreateFromContainer() -> (notimplemented: NotImplemented)
method ImportImage() -> (notimplemented: NotImplemented)
method ExportImage() -> (notimplemented: NotImplemented)
method PullImage() -> (notimplemented: NotImplemented)


# Something failed
error ActionFailed (reason: string)
`
}

type VarlinkInterface struct {
	ioprojectatomicpodmanInterface
}

func VarlinkNew(m ioprojectatomicpodmanInterface) *VarlinkInterface {
	return &VarlinkInterface{m}
}
