// Generated with github.com/varlink/go/cmd/varlink-go-interface-generator
package ioprojectatomicpodman

import "github.com/varlink/go/varlink"

// Type declarations
type Version struct {
	Version    string `json:"version"`
	Go_version string `json:"go_version"`
	Git_commit string `json:"git_commit"`
	Built      int64  `json:"built"`
	Os_arch    string `json:"os_arch"`
}

type NotImplemented struct {
	Comment string `json:"comment"`
}

type StringResponse struct {
	Message string `json:"message"`
}

type ImageInList struct {
	Id          string            `json:"id"`
	ParentId    string            `json:"parentId"`
	RepoTags    []string          `json:"repoTags"`
	RepoDigests []string          `json:"repoDigests"`
	Created     string            `json:"created"`
	Size        int64             `json:"size"`
	VirtualSize int64             `json:"virtualSize"`
	Containers  int64             `json:"containers"`
	Labels      map[string]string `json:"labels"`
}

type ImageHistory struct {
	Id        string   `json:"id"`
	Created   string   `json:"created"`
	CreatedBy string   `json:"createdBy"`
	Tags      []string `json:"tags"`
	Size      int64    `json:"size"`
	Comment   string   `json:"comment"`
}

type ImageSearch struct {
	Description  string `json:"description"`
	Is_official  bool   `json:"is_official"`
	Is_automated bool   `json:"is_automated"`
	Name         string `json:"name"`
	Star_count   int64  `json:"star_count"`
}

// Client method calls and reply readers
func InspectContainer(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.InspectContainer", nil, more__, oneway__)
}

func ReadInspectContainer_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

func ListContainerChanges(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.ListContainerChanges", nil, more__, oneway__)
}

func ReadListContainerChanges_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

func GetContainerStats(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.GetContainerStats", nil, more__, oneway__)
}

func ReadGetContainerStats_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

func InspectImage(c__ *varlink.Connection, more__ bool, oneway__ bool, name_ string) error {
	var in struct {
		Name string `json:"name"`
	}
	in.Name = name_
	return c__.Send("io.projectatomic.podman.InspectImage", in, more__, oneway__)
}

func ReadInspectImage_(c__ *varlink.Connection, image_ *string) (bool, error) {
	var out struct {
		Image string `json:"image"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if image_ != nil {
		*image_ = out.Image
	}
	return continues_, nil
}

func DeleteUnusedImages(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.DeleteUnusedImages", nil, more__, oneway__)
}

func ReadDeleteUnusedImages_(c__ *varlink.Connection, images_ *[]string) (bool, error) {
	var out struct {
		Images []string `json:"images"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if images_ != nil {
		*images_ = []string(out.Images)
	}
	return continues_, nil
}

func ImportImage(c__ *varlink.Connection, more__ bool, oneway__ bool, source_ string, reference_ string, message_ string, changes_ []string) error {
	var in struct {
		Source    string   `json:"source"`
		Reference string   `json:"reference"`
		Message   string   `json:"message"`
		Changes   []string `json:"changes"`
	}
	in.Source = source_
	in.Reference = reference_
	in.Message = message_
	in.Changes = []string(changes_)
	return c__.Send("io.projectatomic.podman.ImportImage", in, more__, oneway__)
}

func ReadImportImage_(c__ *varlink.Connection, id_ *string) (bool, error) {
	var out struct {
		Id string `json:"id"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if id_ != nil {
		*id_ = out.Id
	}
	return continues_, nil
}

func ListContainers(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.ListContainers", nil, more__, oneway__)
}

func ReadListContainers_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

func AttachToContainer(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.AttachToContainer", nil, more__, oneway__)
}

func ReadAttachToContainer_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

func RemoveContainer(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.RemoveContainer", nil, more__, oneway__)
}

func ReadRemoveContainer_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

func KillContainer(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.KillContainer", nil, more__, oneway__)
}

func ReadKillContainer_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

func ResizeContainerTty(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.ResizeContainerTty", nil, more__, oneway__)
}

func ReadResizeContainerTty_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

func StartContainer(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.StartContainer", nil, more__, oneway__)
}

func ReadStartContainer_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

func StopContainer(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.StopContainer", nil, more__, oneway__)
}

func ReadStopContainer_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

func UnpauseContainer(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.UnpauseContainer", nil, more__, oneway__)
}

func ReadUnpauseContainer_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

func GetVersion(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.GetVersion", nil, more__, oneway__)
}

func ReadGetVersion_(c__ *varlink.Connection, version_ *Version) (bool, error) {
	var out struct {
		Version Version `json:"version"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if version_ != nil {
		*version_ = out.Version
	}
	return continues_, nil
}

func Ping(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.Ping", nil, more__, oneway__)
}

func ReadPing_(c__ *varlink.Connection, ping_ *StringResponse) (bool, error) {
	var out struct {
		Ping StringResponse `json:"ping"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if ping_ != nil {
		*ping_ = out.Ping
	}
	return continues_, nil
}

func WaitContainer(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.WaitContainer", nil, more__, oneway__)
}

func ReadWaitContainer_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

func BuildImage(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.BuildImage", nil, more__, oneway__)
}

func ReadBuildImage_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

func HistoryImage(c__ *varlink.Connection, more__ bool, oneway__ bool, name_ string) error {
	var in struct {
		Name string `json:"name"`
	}
	in.Name = name_
	return c__.Send("io.projectatomic.podman.HistoryImage", in, more__, oneway__)
}

func ReadHistoryImage_(c__ *varlink.Connection, history_ *[]ImageHistory) (bool, error) {
	var out struct {
		History []ImageHistory `json:"history"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if history_ != nil {
		*history_ = []ImageHistory(out.History)
	}
	return continues_, nil
}

func PushImage(c__ *varlink.Connection, more__ bool, oneway__ bool, name_ string, tag_ string, tlsverify_ bool) error {
	var in struct {
		Name      string `json:"name"`
		Tag       string `json:"tag"`
		Tlsverify bool   `json:"tlsverify"`
	}
	in.Name = name_
	in.Tag = tag_
	in.Tlsverify = tlsverify_
	return c__.Send("io.projectatomic.podman.PushImage", in, more__, oneway__)
}

func ReadPushImage_(c__ *varlink.Connection) (bool, error) {
	continues_, err := c__.Receive(nil)
	if err != nil {
		return false, err
	}
	return continues_, nil
}

func TagImage(c__ *varlink.Connection, more__ bool, oneway__ bool, name_ string, tagged_ string) error {
	var in struct {
		Name   string `json:"name"`
		Tagged string `json:"tagged"`
	}
	in.Name = name_
	in.Tagged = tagged_
	return c__.Send("io.projectatomic.podman.TagImage", in, more__, oneway__)
}

func ReadTagImage_(c__ *varlink.Connection) (bool, error) {
	continues_, err := c__.Receive(nil)
	if err != nil {
		return false, err
	}
	return continues_, nil
}

func SearchImage(c__ *varlink.Connection, more__ bool, oneway__ bool, name_ string, limit_ int64) error {
	var in struct {
		Name  string `json:"name"`
		Limit int64  `json:"limit"`
	}
	in.Name = name_
	in.Limit = limit_
	return c__.Send("io.projectatomic.podman.SearchImage", in, more__, oneway__)
}

func ReadSearchImage_(c__ *varlink.Connection, images_ *[]ImageSearch) (bool, error) {
	var out struct {
		Images []ImageSearch `json:"images"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if images_ != nil {
		*images_ = []ImageSearch(out.Images)
	}
	return continues_, nil
}

func ExportImage(c__ *varlink.Connection, more__ bool, oneway__ bool, name_ string, destination_ string, compress_ bool) error {
	var in struct {
		Name        string `json:"name"`
		Destination string `json:"destination"`
		Compress    bool   `json:"compress"`
	}
	in.Name = name_
	in.Destination = destination_
	in.Compress = compress_
	return c__.Send("io.projectatomic.podman.ExportImage", in, more__, oneway__)
}

func ReadExportImage_(c__ *varlink.Connection) (bool, error) {
	continues_, err := c__.Receive(nil)
	if err != nil {
		return false, err
	}
	return continues_, nil
}

func RenameContainer(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.RenameContainer", nil, more__, oneway__)
}

func ReadRenameContainer_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

func RemoveImage(c__ *varlink.Connection, more__ bool, oneway__ bool, name_ string, force_ bool) error {
	var in struct {
		Name  string `json:"name"`
		Force bool   `json:"force"`
	}
	in.Name = name_
	in.Force = force_
	return c__.Send("io.projectatomic.podman.RemoveImage", in, more__, oneway__)
}

func ReadRemoveImage_(c__ *varlink.Connection) (bool, error) {
	continues_, err := c__.Receive(nil)
	if err != nil {
		return false, err
	}
	return continues_, nil
}

func ExportContainer(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.ExportContainer", nil, more__, oneway__)
}

func ReadExportContainer_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

func RestartContainer(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.RestartContainer", nil, more__, oneway__)
}

func ReadRestartContainer_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

func UpdateContainer(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.UpdateContainer", nil, more__, oneway__)
}

func ReadUpdateContainer_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

func PauseContainer(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.PauseContainer", nil, more__, oneway__)
}

func ReadPauseContainer_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

func DeleteStoppedContainers(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.DeleteStoppedContainers", nil, more__, oneway__)
}

func ReadDeleteStoppedContainers_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

func CreateImage(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.CreateImage", nil, more__, oneway__)
}

func ReadCreateImage_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

func CreateFromContainer(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.CreateFromContainer", nil, more__, oneway__)
}

func ReadCreateFromContainer_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

func PullImage(c__ *varlink.Connection, more__ bool, oneway__ bool, name_ string) error {
	var in struct {
		Name string `json:"name"`
	}
	in.Name = name_
	return c__.Send("io.projectatomic.podman.PullImage", in, more__, oneway__)
}

func ReadPullImage_(c__ *varlink.Connection, id_ *string) (bool, error) {
	var out struct {
		Id string `json:"id"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if id_ != nil {
		*id_ = out.Id
	}
	return continues_, nil
}

func GetContainerLogs(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.GetContainerLogs", nil, more__, oneway__)
}

func ReadGetContainerLogs_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

func ListContainerProcesses(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.ListContainerProcesses", nil, more__, oneway__)
}

func ReadListContainerProcesses_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

func ListImages(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.ListImages", nil, more__, oneway__)
}

func ReadListImages_(c__ *varlink.Connection, images_ *[]ImageInList) (bool, error) {
	var out struct {
		Images []ImageInList `json:"images"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if images_ != nil {
		*images_ = []ImageInList(out.Images)
	}
	return continues_, nil
}

func CreateContainer(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.CreateContainer", nil, more__, oneway__)
}

func ReadCreateContainer_(c__ *varlink.Connection, notimplemented_ *NotImplemented) (bool, error) {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if notimplemented_ != nil {
		*notimplemented_ = out.Notimplemented
	}
	return continues_, nil
}

// Service interface with all methods
type ioprojectatomicpodmanInterface interface {
	PauseContainer(c__ VarlinkCall) error
	DeleteStoppedContainers(c__ VarlinkCall) error
	CreateImage(c__ VarlinkCall) error
	CreateFromContainer(c__ VarlinkCall) error
	PullImage(c__ VarlinkCall, name_ string) error
	GetContainerLogs(c__ VarlinkCall) error
	RestartContainer(c__ VarlinkCall) error
	UpdateContainer(c__ VarlinkCall) error
	CreateContainer(c__ VarlinkCall) error
	ListContainerProcesses(c__ VarlinkCall) error
	ListImages(c__ VarlinkCall) error
	GetContainerStats(c__ VarlinkCall) error
	InspectImage(c__ VarlinkCall, name_ string) error
	DeleteUnusedImages(c__ VarlinkCall) error
	ImportImage(c__ VarlinkCall, source_ string, reference_ string, message_ string, changes_ []string) error
	ListContainers(c__ VarlinkCall) error
	InspectContainer(c__ VarlinkCall) error
	ListContainerChanges(c__ VarlinkCall) error
	KillContainer(c__ VarlinkCall) error
	AttachToContainer(c__ VarlinkCall) error
	RemoveContainer(c__ VarlinkCall) error
	StopContainer(c__ VarlinkCall) error
	UnpauseContainer(c__ VarlinkCall) error
	GetVersion(c__ VarlinkCall) error
	ResizeContainerTty(c__ VarlinkCall) error
	StartContainer(c__ VarlinkCall) error
	Ping(c__ VarlinkCall) error
	HistoryImage(c__ VarlinkCall, name_ string) error
	PushImage(c__ VarlinkCall, name_ string, tag_ string, tlsverify_ bool) error
	TagImage(c__ VarlinkCall, name_ string, tagged_ string) error
	SearchImage(c__ VarlinkCall, name_ string, limit_ int64) error
	ExportImage(c__ VarlinkCall, name_ string, destination_ string, compress_ bool) error
	RenameContainer(c__ VarlinkCall) error
	WaitContainer(c__ VarlinkCall) error
	BuildImage(c__ VarlinkCall) error
	ExportContainer(c__ VarlinkCall) error
	RemoveImage(c__ VarlinkCall, name_ string, force_ bool) error
}

// Service object with all methods
type VarlinkCall struct{ varlink.Call }

// Reply methods for all varlink errors
func (c__ *VarlinkCall) ReplyActionFailed(reason_ string) error {
	var out struct {
		Reason string `json:"reason"`
	}
	out.Reason = reason_
	return c__.ReplyError("io.projectatomic.podman.ActionFailed", &out)
}

func (c__ *VarlinkCall) ReplyImageNotFound(imagename_ string) error {
	var out struct {
		Imagename string `json:"imagename"`
	}
	out.Imagename = imagename_
	return c__.ReplyError("io.projectatomic.podman.ImageNotFound", &out)
}

func (c__ *VarlinkCall) ReplyErrorOccurred(reason_ string) error {
	var out struct {
		Reason string `json:"reason"`
	}
	out.Reason = reason_
	return c__.ReplyError("io.projectatomic.podman.ErrorOccurred", &out)
}

func (c__ *VarlinkCall) ReplyRuntimeError(reason_ string) error {
	var out struct {
		Reason string `json:"reason"`
	}
	out.Reason = reason_
	return c__.ReplyError("io.projectatomic.podman.RuntimeError", &out)
}

// Reply methods for all varlink methods
func (c__ *VarlinkCall) ReplyGetVersion(version_ Version) error {
	var out struct {
		Version Version `json:"version"`
	}
	out.Version = version_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyResizeContainerTty(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyStartContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyStopContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyUnpauseContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyPing(ping_ StringResponse) error {
	var out struct {
		Ping StringResponse `json:"ping"`
	}
	out.Ping = ping_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplySearchImage(images_ []ImageSearch) error {
	var out struct {
		Images []ImageSearch `json:"images"`
	}
	out.Images = []ImageSearch(images_)
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyExportImage() error {
	return c__.Reply(nil)
}

func (c__ *VarlinkCall) ReplyRenameContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyWaitContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyBuildImage(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyHistoryImage(history_ []ImageHistory) error {
	var out struct {
		History []ImageHistory `json:"history"`
	}
	out.History = []ImageHistory(history_)
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyPushImage() error {
	return c__.Reply(nil)
}

func (c__ *VarlinkCall) ReplyTagImage() error {
	return c__.Reply(nil)
}

func (c__ *VarlinkCall) ReplyExportContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyRemoveImage() error {
	return c__.Reply(nil)
}

func (c__ *VarlinkCall) ReplyCreateFromContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyPullImage(id_ string) error {
	var out struct {
		Id string `json:"id"`
	}
	out.Id = id_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyGetContainerLogs(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyRestartContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyUpdateContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyPauseContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyDeleteStoppedContainers(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyCreateImage(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyCreateContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyListContainerProcesses(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyListImages(images_ []ImageInList) error {
	var out struct {
		Images []ImageInList `json:"images"`
	}
	out.Images = []ImageInList(images_)
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyImportImage(id_ string) error {
	var out struct {
		Id string `json:"id"`
	}
	out.Id = id_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyListContainers(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyInspectContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyListContainerChanges(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyGetContainerStats(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyInspectImage(image_ string) error {
	var out struct {
		Image string `json:"image"`
	}
	out.Image = image_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyDeleteUnusedImages(images_ []string) error {
	var out struct {
		Images []string `json:"images"`
	}
	out.Images = []string(images_)
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyKillContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyAttachToContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyRemoveContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

// Dummy methods for all varlink methods
func (s__ *VarlinkInterface) UnpauseContainer(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("UnpauseContainer")
}

func (s__ *VarlinkInterface) GetVersion(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("GetVersion")
}

func (s__ *VarlinkInterface) ResizeContainerTty(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("ResizeContainerTty")
}

func (s__ *VarlinkInterface) StartContainer(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("StartContainer")
}

func (s__ *VarlinkInterface) StopContainer(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("StopContainer")
}

func (s__ *VarlinkInterface) Ping(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("Ping")
}

func (s__ *VarlinkInterface) PushImage(c__ VarlinkCall, name_ string, tag_ string, tlsverify_ bool) error {
	return c__.ReplyMethodNotImplemented("PushImage")
}

func (s__ *VarlinkInterface) TagImage(c__ VarlinkCall, name_ string, tagged_ string) error {
	return c__.ReplyMethodNotImplemented("TagImage")
}

func (s__ *VarlinkInterface) SearchImage(c__ VarlinkCall, name_ string, limit_ int64) error {
	return c__.ReplyMethodNotImplemented("SearchImage")
}

func (s__ *VarlinkInterface) ExportImage(c__ VarlinkCall, name_ string, destination_ string, compress_ bool) error {
	return c__.ReplyMethodNotImplemented("ExportImage")
}

func (s__ *VarlinkInterface) RenameContainer(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("RenameContainer")
}

func (s__ *VarlinkInterface) WaitContainer(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("WaitContainer")
}

func (s__ *VarlinkInterface) BuildImage(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("BuildImage")
}

func (s__ *VarlinkInterface) HistoryImage(c__ VarlinkCall, name_ string) error {
	return c__.ReplyMethodNotImplemented("HistoryImage")
}

func (s__ *VarlinkInterface) ExportContainer(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("ExportContainer")
}

func (s__ *VarlinkInterface) RemoveImage(c__ VarlinkCall, name_ string, force_ bool) error {
	return c__.ReplyMethodNotImplemented("RemoveImage")
}

func (s__ *VarlinkInterface) DeleteStoppedContainers(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("DeleteStoppedContainers")
}

func (s__ *VarlinkInterface) CreateImage(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("CreateImage")
}

func (s__ *VarlinkInterface) CreateFromContainer(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("CreateFromContainer")
}

func (s__ *VarlinkInterface) PullImage(c__ VarlinkCall, name_ string) error {
	return c__.ReplyMethodNotImplemented("PullImage")
}

func (s__ *VarlinkInterface) GetContainerLogs(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("GetContainerLogs")
}

func (s__ *VarlinkInterface) RestartContainer(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("RestartContainer")
}

func (s__ *VarlinkInterface) UpdateContainer(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("UpdateContainer")
}

func (s__ *VarlinkInterface) PauseContainer(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("PauseContainer")
}

func (s__ *VarlinkInterface) CreateContainer(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("CreateContainer")
}

func (s__ *VarlinkInterface) ListContainerProcesses(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("ListContainerProcesses")
}

func (s__ *VarlinkInterface) ListImages(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("ListImages")
}

func (s__ *VarlinkInterface) InspectImage(c__ VarlinkCall, name_ string) error {
	return c__.ReplyMethodNotImplemented("InspectImage")
}

func (s__ *VarlinkInterface) DeleteUnusedImages(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("DeleteUnusedImages")
}

func (s__ *VarlinkInterface) ImportImage(c__ VarlinkCall, source_ string, reference_ string, message_ string, changes_ []string) error {
	return c__.ReplyMethodNotImplemented("ImportImage")
}

func (s__ *VarlinkInterface) ListContainers(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("ListContainers")
}

func (s__ *VarlinkInterface) InspectContainer(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("InspectContainer")
}

func (s__ *VarlinkInterface) ListContainerChanges(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("ListContainerChanges")
}

func (s__ *VarlinkInterface) GetContainerStats(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("GetContainerStats")
}

func (s__ *VarlinkInterface) KillContainer(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("KillContainer")
}

func (s__ *VarlinkInterface) AttachToContainer(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("AttachToContainer")
}

func (s__ *VarlinkInterface) RemoveContainer(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("RemoveContainer")
}

// Method call dispatcher
func (s__ *VarlinkInterface) VarlinkDispatch(call varlink.Call, methodname string) error {
	switch methodname {
	case "KillContainer":
		return s__.ioprojectatomicpodmanInterface.KillContainer(VarlinkCall{call})

	case "AttachToContainer":
		return s__.ioprojectatomicpodmanInterface.AttachToContainer(VarlinkCall{call})

	case "RemoveContainer":
		return s__.ioprojectatomicpodmanInterface.RemoveContainer(VarlinkCall{call})

	case "GetVersion":
		return s__.ioprojectatomicpodmanInterface.GetVersion(VarlinkCall{call})

	case "ResizeContainerTty":
		return s__.ioprojectatomicpodmanInterface.ResizeContainerTty(VarlinkCall{call})

	case "StartContainer":
		return s__.ioprojectatomicpodmanInterface.StartContainer(VarlinkCall{call})

	case "StopContainer":
		return s__.ioprojectatomicpodmanInterface.StopContainer(VarlinkCall{call})

	case "UnpauseContainer":
		return s__.ioprojectatomicpodmanInterface.UnpauseContainer(VarlinkCall{call})

	case "Ping":
		return s__.ioprojectatomicpodmanInterface.Ping(VarlinkCall{call})

	case "TagImage":
		var in struct {
			Name   string `json:"name"`
			Tagged string `json:"tagged"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.TagImage(VarlinkCall{call}, in.Name, in.Tagged)

	case "SearchImage":
		var in struct {
			Name  string `json:"name"`
			Limit int64  `json:"limit"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.SearchImage(VarlinkCall{call}, in.Name, in.Limit)

	case "ExportImage":
		var in struct {
			Name        string `json:"name"`
			Destination string `json:"destination"`
			Compress    bool   `json:"compress"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.ExportImage(VarlinkCall{call}, in.Name, in.Destination, in.Compress)

	case "RenameContainer":
		return s__.ioprojectatomicpodmanInterface.RenameContainer(VarlinkCall{call})

	case "WaitContainer":
		return s__.ioprojectatomicpodmanInterface.WaitContainer(VarlinkCall{call})

	case "BuildImage":
		return s__.ioprojectatomicpodmanInterface.BuildImage(VarlinkCall{call})

	case "HistoryImage":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.HistoryImage(VarlinkCall{call}, in.Name)

	case "PushImage":
		var in struct {
			Name      string `json:"name"`
			Tag       string `json:"tag"`
			Tlsverify bool   `json:"tlsverify"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.PushImage(VarlinkCall{call}, in.Name, in.Tag, in.Tlsverify)

	case "ExportContainer":
		return s__.ioprojectatomicpodmanInterface.ExportContainer(VarlinkCall{call})

	case "RemoveImage":
		var in struct {
			Name  string `json:"name"`
			Force bool   `json:"force"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.RemoveImage(VarlinkCall{call}, in.Name, in.Force)

	case "CreateImage":
		return s__.ioprojectatomicpodmanInterface.CreateImage(VarlinkCall{call})

	case "CreateFromContainer":
		return s__.ioprojectatomicpodmanInterface.CreateFromContainer(VarlinkCall{call})

	case "PullImage":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.PullImage(VarlinkCall{call}, in.Name)

	case "GetContainerLogs":
		return s__.ioprojectatomicpodmanInterface.GetContainerLogs(VarlinkCall{call})

	case "RestartContainer":
		return s__.ioprojectatomicpodmanInterface.RestartContainer(VarlinkCall{call})

	case "UpdateContainer":
		return s__.ioprojectatomicpodmanInterface.UpdateContainer(VarlinkCall{call})

	case "PauseContainer":
		return s__.ioprojectatomicpodmanInterface.PauseContainer(VarlinkCall{call})

	case "DeleteStoppedContainers":
		return s__.ioprojectatomicpodmanInterface.DeleteStoppedContainers(VarlinkCall{call})

	case "CreateContainer":
		return s__.ioprojectatomicpodmanInterface.CreateContainer(VarlinkCall{call})

	case "ListContainerProcesses":
		return s__.ioprojectatomicpodmanInterface.ListContainerProcesses(VarlinkCall{call})

	case "ListImages":
		return s__.ioprojectatomicpodmanInterface.ListImages(VarlinkCall{call})

	case "DeleteUnusedImages":
		return s__.ioprojectatomicpodmanInterface.DeleteUnusedImages(VarlinkCall{call})

	case "ImportImage":
		var in struct {
			Source    string   `json:"source"`
			Reference string   `json:"reference"`
			Message   string   `json:"message"`
			Changes   []string `json:"changes"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.ImportImage(VarlinkCall{call}, in.Source, in.Reference, in.Message, []string(in.Changes))

	case "ListContainers":
		return s__.ioprojectatomicpodmanInterface.ListContainers(VarlinkCall{call})

	case "InspectContainer":
		return s__.ioprojectatomicpodmanInterface.InspectContainer(VarlinkCall{call})

	case "ListContainerChanges":
		return s__.ioprojectatomicpodmanInterface.ListContainerChanges(VarlinkCall{call})

	case "GetContainerStats":
		return s__.ioprojectatomicpodmanInterface.GetContainerStats(VarlinkCall{call})

	case "InspectImage":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.InspectImage(VarlinkCall{call}, in.Name)

	default:
		return call.ReplyMethodNotFound(methodname)
	}
}

// Varlink interface name
func (s__ *VarlinkInterface) VarlinkGetName() string {
	return `io.projectatomic.podman`
}

// Varlink interface description
func (s__ *VarlinkInterface) VarlinkGetDescription() string {
	return `# Podman Service Interface
interface io.projectatomic.podman


# Version is the structure returned by GetVersion
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

# ImageInList describes the structure that is returned in
# ListImages.
type ImageInList (
  id: string,
  parentId: string,
  repoTags: []string,
  repoDigests: []string,
  created: string,
  size: int,
  virtualSize: int,
  containers: int,
  labels: [string]string
)

# ImageHistory describes the returned structure from ImageHistory.
type ImageHistory (
    id: string,
    created: string,
    createdBy: string,
    tags: []string,
    size: int,
    comment: string
)

# ImageSearch is the returned structure for SearchImage.  It is returned
# in arrary form.
type ImageSearch (
    description: string,
    is_official: bool,
    is_automated: bool,
    name: string,
    star_count: int
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
method ListImages() -> (images: []ImageInList)
method BuildImage() -> (notimplemented: NotImplemented)
method CreateImage() -> (notimplemented: NotImplemented)
method InspectImage(name: string) -> (image: string)
method HistoryImage(name: string) -> (history: []ImageHistory)
method PushImage(name: string, tag: string, tlsverify: bool) -> ()
method TagImage(name: string, tagged: string) -> ()
method RemoveImage(name: string, force: bool) -> ()
method SearchImage(name: string, limit: int) -> (images: []ImageSearch)
method DeleteUnusedImages() -> (images: []string)
method CreateFromContainer() -> (notimplemented: NotImplemented)
method ImportImage(source: string, reference: string, message: string, changes: []string) -> (id: string)
method ExportImage(name: string, destination: string, compress: bool) -> ()
method PullImage(name: string) -> (id: string)


# Something failed
error ActionFailed (reason: string)
error ImageNotFound (imagename: string)
error ErrorOccurred (reason: string)
error RuntimeError (reason: string)
`
}

// Service interface
type VarlinkInterface struct {
	ioprojectatomicpodmanInterface
}

func VarlinkNew(m ioprojectatomicpodmanInterface) *VarlinkInterface {
	return &VarlinkInterface{m}
}
