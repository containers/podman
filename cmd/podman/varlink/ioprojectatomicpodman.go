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

type ContainerStats struct {
	Id           string  `json:"id"`
	Name         string  `json:"name"`
	Cpu          float64 `json:"cpu"`
	Cpu_nano     int64   `json:"cpu_nano"`
	System_nano  int64   `json:"system_nano"`
	Mem_usage    int64   `json:"mem_usage"`
	Mem_limit    int64   `json:"mem_limit"`
	Mem_perc     float64 `json:"mem_perc"`
	Net_input    int64   `json:"net_input"`
	Net_output   int64   `json:"net_output"`
	Block_output int64   `json:"block_output"`
	Block_input  int64   `json:"block_input"`
	Pids         int64   `json:"pids"`
}

type ContainerPortMappings struct {
	Host_port      string `json:"host_port"`
	Host_ip        string `json:"host_ip"`
	Protocol       string `json:"protocol"`
	Container_port string `json:"container_port"`
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

type ListContainerData struct {
	Id               string                  `json:"id"`
	Image            string                  `json:"image"`
	Imageid          string                  `json:"imageid"`
	Command          []string                `json:"command"`
	Createdat        string                  `json:"createdat"`
	Runningfor       string                  `json:"runningfor"`
	Status           string                  `json:"status"`
	Ports            []ContainerPortMappings `json:"ports"`
	Rootfssize       int64                   `json:"rootfssize"`
	Rwsize           int64                   `json:"rwsize"`
	Names            string                  `json:"names"`
	Labels           map[string]string       `json:"labels"`
	Mounts           []ContainerMount        `json:"mounts"`
	Containerrunning bool                    `json:"containerrunning"`
	Namespaces       ContainerNameSpace      `json:"namespaces"`
}

type ContainerMount struct {
	Destination string   `json:"destination"`
	Type        string   `json:"type"`
	Source      string   `json:"source"`
	Options     []string `json:"options"`
}

type ContainerNameSpace struct {
	User   string `json:"user"`
	Uts    string `json:"uts"`
	Pidns  string `json:"pidns"`
	Pid    string `json:"pid"`
	Cgroup string `json:"cgroup"`
	Net    string `json:"net"`
	Mnt    string `json:"mnt"`
	Ipc    string `json:"ipc"`
}

// Client method calls and reply readers
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

func ExportContainer(c__ *varlink.Connection, more__ bool, oneway__ bool, name_ string, path_ string) error {
	var in struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	in.Name = name_
	in.Path = path_
	return c__.Send("io.projectatomic.podman.ExportContainer", in, more__, oneway__)
}

func ReadExportContainer_(c__ *varlink.Connection, tarfile_ *string) (bool, error) {
	var out struct {
		Tarfile string `json:"tarfile"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if tarfile_ != nil {
		*tarfile_ = out.Tarfile
	}
	return continues_, nil
}

func RestartContainer(c__ *varlink.Connection, more__ bool, oneway__ bool, name_ string, timeout_ int64) error {
	var in struct {
		Name    string `json:"name"`
		Timeout int64  `json:"timeout"`
	}
	in.Name = name_
	in.Timeout = timeout_
	return c__.Send("io.projectatomic.podman.RestartContainer", in, more__, oneway__)
}

func ReadRestartContainer_(c__ *varlink.Connection, container_ *string) (bool, error) {
	var out struct {
		Container string `json:"container"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if container_ != nil {
		*container_ = out.Container
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

func PauseContainer(c__ *varlink.Connection, more__ bool, oneway__ bool, name_ string) error {
	var in struct {
		Name string `json:"name"`
	}
	in.Name = name_
	return c__.Send("io.projectatomic.podman.PauseContainer", in, more__, oneway__)
}

func ReadPauseContainer_(c__ *varlink.Connection, container_ *string) (bool, error) {
	var out struct {
		Container string `json:"container"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if container_ != nil {
		*container_ = out.Container
	}
	return continues_, nil
}

func WaitContainer(c__ *varlink.Connection, more__ bool, oneway__ bool, name_ string) error {
	var in struct {
		Name string `json:"name"`
	}
	in.Name = name_
	return c__.Send("io.projectatomic.podman.WaitContainer", in, more__, oneway__)
}

func ReadWaitContainer_(c__ *varlink.Connection, exitcode_ *int64) (bool, error) {
	var out struct {
		Exitcode int64 `json:"exitcode"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if exitcode_ != nil {
		*exitcode_ = out.Exitcode
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

func ListContainerProcesses(c__ *varlink.Connection, more__ bool, oneway__ bool, name_ string, opts_ []string) error {
	var in struct {
		Name string   `json:"name"`
		Opts []string `json:"opts"`
	}
	in.Name = name_
	in.Opts = []string(opts_)
	return c__.Send("io.projectatomic.podman.ListContainerProcesses", in, more__, oneway__)
}

func ReadListContainerProcesses_(c__ *varlink.Connection, container_ *[]string) (bool, error) {
	var out struct {
		Container []string `json:"container"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if container_ != nil {
		*container_ = []string(out.Container)
	}
	return continues_, nil
}

func ListContainerChanges(c__ *varlink.Connection, more__ bool, oneway__ bool, name_ string) error {
	var in struct {
		Name string `json:"name"`
	}
	in.Name = name_
	return c__.Send("io.projectatomic.podman.ListContainerChanges", in, more__, oneway__)
}

func ReadListContainerChanges_(c__ *varlink.Connection, container_ *map[string]string) (bool, error) {
	var out struct {
		Container map[string]string `json:"container"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if container_ != nil {
		*container_ = map[string]string(out.Container)
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

func ReadRemoveImage_(c__ *varlink.Connection, image_ *string) (bool, error) {
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

func ReadImportImage_(c__ *varlink.Connection, image_ *string) (bool, error) {
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

func GetContainerLogs(c__ *varlink.Connection, more__ bool, oneway__ bool, name_ string) error {
	var in struct {
		Name string `json:"name"`
	}
	in.Name = name_
	return c__.Send("io.projectatomic.podman.GetContainerLogs", in, more__, oneway__)
}

func ReadGetContainerLogs_(c__ *varlink.Connection, container_ *[]string) (bool, error) {
	var out struct {
		Container []string `json:"container"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if container_ != nil {
		*container_ = []string(out.Container)
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

func KillContainer(c__ *varlink.Connection, more__ bool, oneway__ bool, name_ string, signal_ int64) error {
	var in struct {
		Name   string `json:"name"`
		Signal int64  `json:"signal"`
	}
	in.Name = name_
	in.Signal = signal_
	return c__.Send("io.projectatomic.podman.KillContainer", in, more__, oneway__)
}

func ReadKillContainer_(c__ *varlink.Connection, container_ *string) (bool, error) {
	var out struct {
		Container string `json:"container"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if container_ != nil {
		*container_ = out.Container
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

func GetContainer(c__ *varlink.Connection, more__ bool, oneway__ bool, name_ string) error {
	var in struct {
		Name string `json:"name"`
	}
	in.Name = name_
	return c__.Send("io.projectatomic.podman.GetContainer", in, more__, oneway__)
}

func ReadGetContainer_(c__ *varlink.Connection, container_ *ListContainerData) (bool, error) {
	var out struct {
		Container ListContainerData `json:"container"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if container_ != nil {
		*container_ = out.Container
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

func RemoveContainer(c__ *varlink.Connection, more__ bool, oneway__ bool, name_ string, force_ bool) error {
	var in struct {
		Name  string `json:"name"`
		Force bool   `json:"force"`
	}
	in.Name = name_
	in.Force = force_
	return c__.Send("io.projectatomic.podman.RemoveContainer", in, more__, oneway__)
}

func ReadRemoveContainer_(c__ *varlink.Connection, container_ *string) (bool, error) {
	var out struct {
		Container string `json:"container"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if container_ != nil {
		*container_ = out.Container
	}
	return continues_, nil
}

func DeleteStoppedContainers(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.DeleteStoppedContainers", nil, more__, oneway__)
}

func ReadDeleteStoppedContainers_(c__ *varlink.Connection, containers_ *[]string) (bool, error) {
	var out struct {
		Containers []string `json:"containers"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if containers_ != nil {
		*containers_ = []string(out.Containers)
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

func GetContainerStats(c__ *varlink.Connection, more__ bool, oneway__ bool, name_ string) error {
	var in struct {
		Name string `json:"name"`
	}
	in.Name = name_
	return c__.Send("io.projectatomic.podman.GetContainerStats", in, more__, oneway__)
}

func ReadGetContainerStats_(c__ *varlink.Connection, container_ *ContainerStats) (bool, error) {
	var out struct {
		Container ContainerStats `json:"container"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if container_ != nil {
		*container_ = out.Container
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

func InspectContainer(c__ *varlink.Connection, more__ bool, oneway__ bool, name_ string) error {
	var in struct {
		Name string `json:"name"`
	}
	in.Name = name_
	return c__.Send("io.projectatomic.podman.InspectContainer", in, more__, oneway__)
}

func ReadInspectContainer_(c__ *varlink.Connection, container_ *string) (bool, error) {
	var out struct {
		Container string `json:"container"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if container_ != nil {
		*container_ = out.Container
	}
	return continues_, nil
}

func StopContainer(c__ *varlink.Connection, more__ bool, oneway__ bool, name_ string, timeout_ int64) error {
	var in struct {
		Name    string `json:"name"`
		Timeout int64  `json:"timeout"`
	}
	in.Name = name_
	in.Timeout = timeout_
	return c__.Send("io.projectatomic.podman.StopContainer", in, more__, oneway__)
}

func ReadStopContainer_(c__ *varlink.Connection, container_ *string) (bool, error) {
	var out struct {
		Container string `json:"container"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if container_ != nil {
		*container_ = out.Container
	}
	return continues_, nil
}

func UnpauseContainer(c__ *varlink.Connection, more__ bool, oneway__ bool, name_ string) error {
	var in struct {
		Name string `json:"name"`
	}
	in.Name = name_
	return c__.Send("io.projectatomic.podman.UnpauseContainer", in, more__, oneway__)
}

func ReadUnpauseContainer_(c__ *varlink.Connection, container_ *string) (bool, error) {
	var out struct {
		Container string `json:"container"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if container_ != nil {
		*container_ = out.Container
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

func ListContainers(c__ *varlink.Connection, more__ bool, oneway__ bool) error {
	return c__.Send("io.projectatomic.podman.ListContainers", nil, more__, oneway__)
}

func ReadListContainers_(c__ *varlink.Connection, containers_ *[]ListContainerData) (bool, error) {
	var out struct {
		Containers []ListContainerData `json:"containers"`
	}
	continues_, err := c__.Receive(&out)
	if err != nil {
		return false, err
	}
	if containers_ != nil {
		*containers_ = []ListContainerData(out.Containers)
	}
	return continues_, nil
}

// Service interface with all methods
type ioprojectatomicpodmanInterface interface {
	PauseContainer(c__ VarlinkCall, name_ string) error
	WaitContainer(c__ VarlinkCall, name_ string) error
	ListImages(c__ VarlinkCall) error
	PullImage(c__ VarlinkCall, name_ string) error
	ListContainerProcesses(c__ VarlinkCall, name_ string, opts_ []string) error
	ListContainerChanges(c__ VarlinkCall, name_ string) error
	StartContainer(c__ VarlinkCall) error
	AttachToContainer(c__ VarlinkCall) error
	GetContainerLogs(c__ VarlinkCall, name_ string) error
	BuildImage(c__ VarlinkCall) error
	RemoveImage(c__ VarlinkCall, name_ string, force_ bool) error
	ImportImage(c__ VarlinkCall, source_ string, reference_ string, message_ string, changes_ []string) error
	SearchImage(c__ VarlinkCall, name_ string, limit_ int64) error
	DeleteUnusedImages(c__ VarlinkCall) error
	Ping(c__ VarlinkCall) error
	GetContainer(c__ VarlinkCall, name_ string) error
	ResizeContainerTty(c__ VarlinkCall) error
	KillContainer(c__ VarlinkCall, name_ string, signal_ int64) error
	RenameContainer(c__ VarlinkCall) error
	CreateImage(c__ VarlinkCall) error
	InspectImage(c__ VarlinkCall, name_ string) error
	PushImage(c__ VarlinkCall, name_ string, tag_ string, tlsverify_ bool) error
	ExportImage(c__ VarlinkCall, name_ string, destination_ string, compress_ bool) error
	GetContainerStats(c__ VarlinkCall, name_ string) error
	UpdateContainer(c__ VarlinkCall) error
	RemoveContainer(c__ VarlinkCall, name_ string, force_ bool) error
	DeleteStoppedContainers(c__ VarlinkCall) error
	StopContainer(c__ VarlinkCall, name_ string, timeout_ int64) error
	UnpauseContainer(c__ VarlinkCall, name_ string) error
	CreateFromContainer(c__ VarlinkCall) error
	GetVersion(c__ VarlinkCall) error
	ListContainers(c__ VarlinkCall) error
	CreateContainer(c__ VarlinkCall) error
	InspectContainer(c__ VarlinkCall, name_ string) error
	ExportContainer(c__ VarlinkCall, name_ string, path_ string) error
	RestartContainer(c__ VarlinkCall, name_ string, timeout_ int64) error
	HistoryImage(c__ VarlinkCall, name_ string) error
	TagImage(c__ VarlinkCall, name_ string, tagged_ string) error
}

// Service object with all methods
type VarlinkCall struct{ varlink.Call }

// Reply methods for all varlink errors
func (c__ *VarlinkCall) ReplyImageNotFound(name_ string) error {
	var out struct {
		Name string `json:"name"`
	}
	out.Name = name_
	return c__.ReplyError("io.projectatomic.podman.ImageNotFound", &out)
}

func (c__ *VarlinkCall) ReplyContainerNotFound(name_ string) error {
	var out struct {
		Name string `json:"name"`
	}
	out.Name = name_
	return c__.ReplyError("io.projectatomic.podman.ContainerNotFound", &out)
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

func (c__ *VarlinkCall) ReplyActionFailed(reason_ string) error {
	var out struct {
		Reason string `json:"reason"`
	}
	out.Reason = reason_
	return c__.ReplyError("io.projectatomic.podman.ActionFailed", &out)
}

// Reply methods for all varlink methods
func (c__ *VarlinkCall) ReplyKillContainer(container_ string) error {
	var out struct {
		Container string `json:"container"`
	}
	out.Container = container_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplySearchImage(images_ []ImageSearch) error {
	var out struct {
		Images []ImageSearch `json:"images"`
	}
	out.Images = []ImageSearch(images_)
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyDeleteUnusedImages(images_ []string) error {
	var out struct {
		Images []string `json:"images"`
	}
	out.Images = []string(images_)
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyPing(ping_ StringResponse) error {
	var out struct {
		Ping StringResponse `json:"ping"`
	}
	out.Ping = ping_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyGetContainer(container_ ListContainerData) error {
	var out struct {
		Container ListContainerData `json:"container"`
	}
	out.Container = container_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyResizeContainerTty(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyRenameContainer(notimplemented_ NotImplemented) error {
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

func (c__ *VarlinkCall) ReplyInspectImage(image_ string) error {
	var out struct {
		Image string `json:"image"`
	}
	out.Image = image_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyDeleteStoppedContainers(containers_ []string) error {
	var out struct {
		Containers []string `json:"containers"`
	}
	out.Containers = []string(containers_)
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyPushImage() error {
	return c__.Reply(nil)
}

func (c__ *VarlinkCall) ReplyExportImage() error {
	return c__.Reply(nil)
}

func (c__ *VarlinkCall) ReplyGetContainerStats(container_ ContainerStats) error {
	var out struct {
		Container ContainerStats `json:"container"`
	}
	out.Container = container_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyUpdateContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyRemoveContainer(container_ string) error {
	var out struct {
		Container string `json:"container"`
	}
	out.Container = container_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyInspectContainer(container_ string) error {
	var out struct {
		Container string `json:"container"`
	}
	out.Container = container_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyStopContainer(container_ string) error {
	var out struct {
		Container string `json:"container"`
	}
	out.Container = container_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyUnpauseContainer(container_ string) error {
	var out struct {
		Container string `json:"container"`
	}
	out.Container = container_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyCreateFromContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyGetVersion(version_ Version) error {
	var out struct {
		Version Version `json:"version"`
	}
	out.Version = version_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyListContainers(containers_ []ListContainerData) error {
	var out struct {
		Containers []ListContainerData `json:"containers"`
	}
	out.Containers = []ListContainerData(containers_)
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyCreateContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyTagImage() error {
	return c__.Reply(nil)
}

func (c__ *VarlinkCall) ReplyExportContainer(tarfile_ string) error {
	var out struct {
		Tarfile string `json:"tarfile"`
	}
	out.Tarfile = tarfile_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyRestartContainer(container_ string) error {
	var out struct {
		Container string `json:"container"`
	}
	out.Container = container_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyHistoryImage(history_ []ImageHistory) error {
	var out struct {
		History []ImageHistory `json:"history"`
	}
	out.History = []ImageHistory(history_)
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyPullImage(id_ string) error {
	var out struct {
		Id string `json:"id"`
	}
	out.Id = id_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyPauseContainer(container_ string) error {
	var out struct {
		Container string `json:"container"`
	}
	out.Container = container_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyWaitContainer(exitcode_ int64) error {
	var out struct {
		Exitcode int64 `json:"exitcode"`
	}
	out.Exitcode = exitcode_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyListImages(images_ []ImageInList) error {
	var out struct {
		Images []ImageInList `json:"images"`
	}
	out.Images = []ImageInList(images_)
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyAttachToContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyListContainerProcesses(container_ []string) error {
	var out struct {
		Container []string `json:"container"`
	}
	out.Container = []string(container_)
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyListContainerChanges(container_ map[string]string) error {
	var out struct {
		Container map[string]string `json:"container"`
	}
	out.Container = map[string]string(container_)
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyStartContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyImportImage(image_ string) error {
	var out struct {
		Image string `json:"image"`
	}
	out.Image = image_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyGetContainerLogs(container_ []string) error {
	var out struct {
		Container []string `json:"container"`
	}
	out.Container = []string(container_)
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyBuildImage(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c__.Reply(&out)
}

func (c__ *VarlinkCall) ReplyRemoveImage(image_ string) error {
	var out struct {
		Image string `json:"image"`
	}
	out.Image = image_
	return c__.Reply(&out)
}

// Dummy methods for all varlink methods
func (s__ *VarlinkInterface) PauseContainer(c__ VarlinkCall, name_ string) error {
	return c__.ReplyMethodNotImplemented("PauseContainer")
}

func (s__ *VarlinkInterface) WaitContainer(c__ VarlinkCall, name_ string) error {
	return c__.ReplyMethodNotImplemented("WaitContainer")
}

func (s__ *VarlinkInterface) ListImages(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("ListImages")
}

func (s__ *VarlinkInterface) PullImage(c__ VarlinkCall, name_ string) error {
	return c__.ReplyMethodNotImplemented("PullImage")
}

func (s__ *VarlinkInterface) ListContainerProcesses(c__ VarlinkCall, name_ string, opts_ []string) error {
	return c__.ReplyMethodNotImplemented("ListContainerProcesses")
}

func (s__ *VarlinkInterface) ListContainerChanges(c__ VarlinkCall, name_ string) error {
	return c__.ReplyMethodNotImplemented("ListContainerChanges")
}

func (s__ *VarlinkInterface) StartContainer(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("StartContainer")
}

func (s__ *VarlinkInterface) AttachToContainer(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("AttachToContainer")
}

func (s__ *VarlinkInterface) GetContainerLogs(c__ VarlinkCall, name_ string) error {
	return c__.ReplyMethodNotImplemented("GetContainerLogs")
}

func (s__ *VarlinkInterface) BuildImage(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("BuildImage")
}

func (s__ *VarlinkInterface) RemoveImage(c__ VarlinkCall, name_ string, force_ bool) error {
	return c__.ReplyMethodNotImplemented("RemoveImage")
}

func (s__ *VarlinkInterface) ImportImage(c__ VarlinkCall, source_ string, reference_ string, message_ string, changes_ []string) error {
	return c__.ReplyMethodNotImplemented("ImportImage")
}

func (s__ *VarlinkInterface) Ping(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("Ping")
}

func (s__ *VarlinkInterface) GetContainer(c__ VarlinkCall, name_ string) error {
	return c__.ReplyMethodNotImplemented("GetContainer")
}

func (s__ *VarlinkInterface) ResizeContainerTty(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("ResizeContainerTty")
}

func (s__ *VarlinkInterface) KillContainer(c__ VarlinkCall, name_ string, signal_ int64) error {
	return c__.ReplyMethodNotImplemented("KillContainer")
}

func (s__ *VarlinkInterface) SearchImage(c__ VarlinkCall, name_ string, limit_ int64) error {
	return c__.ReplyMethodNotImplemented("SearchImage")
}

func (s__ *VarlinkInterface) DeleteUnusedImages(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("DeleteUnusedImages")
}

func (s__ *VarlinkInterface) RenameContainer(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("RenameContainer")
}

func (s__ *VarlinkInterface) CreateImage(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("CreateImage")
}

func (s__ *VarlinkInterface) InspectImage(c__ VarlinkCall, name_ string) error {
	return c__.ReplyMethodNotImplemented("InspectImage")
}

func (s__ *VarlinkInterface) GetContainerStats(c__ VarlinkCall, name_ string) error {
	return c__.ReplyMethodNotImplemented("GetContainerStats")
}

func (s__ *VarlinkInterface) UpdateContainer(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("UpdateContainer")
}

func (s__ *VarlinkInterface) RemoveContainer(c__ VarlinkCall, name_ string, force_ bool) error {
	return c__.ReplyMethodNotImplemented("RemoveContainer")
}

func (s__ *VarlinkInterface) DeleteStoppedContainers(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("DeleteStoppedContainers")
}

func (s__ *VarlinkInterface) PushImage(c__ VarlinkCall, name_ string, tag_ string, tlsverify_ bool) error {
	return c__.ReplyMethodNotImplemented("PushImage")
}

func (s__ *VarlinkInterface) ExportImage(c__ VarlinkCall, name_ string, destination_ string, compress_ bool) error {
	return c__.ReplyMethodNotImplemented("ExportImage")
}

func (s__ *VarlinkInterface) GetVersion(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("GetVersion")
}

func (s__ *VarlinkInterface) ListContainers(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("ListContainers")
}

func (s__ *VarlinkInterface) CreateContainer(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("CreateContainer")
}

func (s__ *VarlinkInterface) InspectContainer(c__ VarlinkCall, name_ string) error {
	return c__.ReplyMethodNotImplemented("InspectContainer")
}

func (s__ *VarlinkInterface) StopContainer(c__ VarlinkCall, name_ string, timeout_ int64) error {
	return c__.ReplyMethodNotImplemented("StopContainer")
}

func (s__ *VarlinkInterface) UnpauseContainer(c__ VarlinkCall, name_ string) error {
	return c__.ReplyMethodNotImplemented("UnpauseContainer")
}

func (s__ *VarlinkInterface) CreateFromContainer(c__ VarlinkCall) error {
	return c__.ReplyMethodNotImplemented("CreateFromContainer")
}

func (s__ *VarlinkInterface) ExportContainer(c__ VarlinkCall, name_ string, path_ string) error {
	return c__.ReplyMethodNotImplemented("ExportContainer")
}

func (s__ *VarlinkInterface) RestartContainer(c__ VarlinkCall, name_ string, timeout_ int64) error {
	return c__.ReplyMethodNotImplemented("RestartContainer")
}

func (s__ *VarlinkInterface) HistoryImage(c__ VarlinkCall, name_ string) error {
	return c__.ReplyMethodNotImplemented("HistoryImage")
}

func (s__ *VarlinkInterface) TagImage(c__ VarlinkCall, name_ string, tagged_ string) error {
	return c__.ReplyMethodNotImplemented("TagImage")
}

// Method call dispatcher
func (s__ *VarlinkInterface) VarlinkDispatch(call varlink.Call, methodname string) error {
	switch methodname {
	case "GetContainerLogs":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.GetContainerLogs(VarlinkCall{call}, in.Name)

	case "BuildImage":
		return s__.ioprojectatomicpodmanInterface.BuildImage(VarlinkCall{call})

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

	case "Ping":
		return s__.ioprojectatomicpodmanInterface.Ping(VarlinkCall{call})

	case "GetContainer":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.GetContainer(VarlinkCall{call}, in.Name)

	case "ResizeContainerTty":
		return s__.ioprojectatomicpodmanInterface.ResizeContainerTty(VarlinkCall{call})

	case "KillContainer":
		var in struct {
			Name   string `json:"name"`
			Signal int64  `json:"signal"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.KillContainer(VarlinkCall{call}, in.Name, in.Signal)

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

	case "DeleteUnusedImages":
		return s__.ioprojectatomicpodmanInterface.DeleteUnusedImages(VarlinkCall{call})

	case "RenameContainer":
		return s__.ioprojectatomicpodmanInterface.RenameContainer(VarlinkCall{call})

	case "CreateImage":
		return s__.ioprojectatomicpodmanInterface.CreateImage(VarlinkCall{call})

	case "InspectImage":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.InspectImage(VarlinkCall{call}, in.Name)

	case "GetContainerStats":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.GetContainerStats(VarlinkCall{call}, in.Name)

	case "UpdateContainer":
		return s__.ioprojectatomicpodmanInterface.UpdateContainer(VarlinkCall{call})

	case "RemoveContainer":
		var in struct {
			Name  string `json:"name"`
			Force bool   `json:"force"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.RemoveContainer(VarlinkCall{call}, in.Name, in.Force)

	case "DeleteStoppedContainers":
		return s__.ioprojectatomicpodmanInterface.DeleteStoppedContainers(VarlinkCall{call})

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

	case "CreateFromContainer":
		return s__.ioprojectatomicpodmanInterface.CreateFromContainer(VarlinkCall{call})

	case "GetVersion":
		return s__.ioprojectatomicpodmanInterface.GetVersion(VarlinkCall{call})

	case "ListContainers":
		return s__.ioprojectatomicpodmanInterface.ListContainers(VarlinkCall{call})

	case "CreateContainer":
		return s__.ioprojectatomicpodmanInterface.CreateContainer(VarlinkCall{call})

	case "InspectContainer":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.InspectContainer(VarlinkCall{call}, in.Name)

	case "StopContainer":
		var in struct {
			Name    string `json:"name"`
			Timeout int64  `json:"timeout"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.StopContainer(VarlinkCall{call}, in.Name, in.Timeout)

	case "UnpauseContainer":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.UnpauseContainer(VarlinkCall{call}, in.Name)

	case "ExportContainer":
		var in struct {
			Name string `json:"name"`
			Path string `json:"path"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.ExportContainer(VarlinkCall{call}, in.Name, in.Path)

	case "RestartContainer":
		var in struct {
			Name    string `json:"name"`
			Timeout int64  `json:"timeout"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.RestartContainer(VarlinkCall{call}, in.Name, in.Timeout)

	case "HistoryImage":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.HistoryImage(VarlinkCall{call}, in.Name)

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

	case "PauseContainer":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.PauseContainer(VarlinkCall{call}, in.Name)

	case "WaitContainer":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.WaitContainer(VarlinkCall{call}, in.Name)

	case "ListImages":
		return s__.ioprojectatomicpodmanInterface.ListImages(VarlinkCall{call})

	case "PullImage":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.PullImage(VarlinkCall{call}, in.Name)

	case "ListContainerProcesses":
		var in struct {
			Name string   `json:"name"`
			Opts []string `json:"opts"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.ListContainerProcesses(VarlinkCall{call}, in.Name, []string(in.Opts))

	case "ListContainerChanges":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s__.ioprojectatomicpodmanInterface.ListContainerChanges(VarlinkCall{call}, in.Name)

	case "StartContainer":
		return s__.ioprojectatomicpodmanInterface.StartContainer(VarlinkCall{call})

	case "AttachToContainer":
		return s__.ioprojectatomicpodmanInterface.AttachToContainer(VarlinkCall{call})

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

# ListContainer is the returned struct for an individual container
type ListContainerData (
    id: string,
    image: string,
    imageid: string,
    command: []string,
    createdat: string,
    runningfor: string,
    status: string,
    ports: []ContainerPortMappings,
    rootfssize: int,
    rwsize: int,
    names: string,
    labels: [string]string,
    mounts: []ContainerMount,
    containerrunning: bool,
    namespaces: ContainerNameSpace
)

# ContainerStats is the return struct for the stats of a container
type ContainerStats (
    id: string,
    name: string,
    cpu: float,
    cpu_nano: int,
    system_nano: int,
    mem_usage: int,
    mem_limit: int,
    mem_perc: float,
    net_input: int,
    net_output: int,
    block_output: int,
    block_input: int,
    pids: int
)

# ContainerMount describes the struct for mounts in a container
type ContainerMount (
    destination: string,
    type: string,
    source: string,
    options: []string
)

# ContainerPortMappings describes the struct for portmappings in an existing container
type ContainerPortMappings (
    host_port: string,
    host_ip: string,
    protocol: string,
    container_port: string
)

# ContainerNamespace describes the namespace structure for an existing container
type ContainerNameSpace (
    user: string,
    uts: string,
    pidns: string,
    pid: string,
    cgroup: string,
    net: string,
    mnt: string,
    ipc: string
)

# System
method Ping() -> (ping: StringResponse)
method GetVersion() -> (version: Version)

# Containers
method ListContainers() -> (containers: []ListContainerData)
method GetContainer(name: string) -> (container: ListContainerData)
method CreateContainer() -> (notimplemented: NotImplemented)
method InspectContainer(name: string) -> (container: string)
# TODO: Should this be made into a streaming response as opposed to a one off?
method ListContainerProcesses(name: string, opts: []string) -> (container: []string)
# TODO: Should this be made into a streaming response as opposed to a one off?
method GetContainerLogs(name: string) -> (container: []string)
method ListContainerChanges(name: string) -> (container: [string]string)
# TODO: This should be made into a streaming response
method ExportContainer(name: string, path: string) -> (tarfile: string)
method GetContainerStats(name: string) -> (container: ContainerStats)
method ResizeContainerTty() -> (notimplemented: NotImplemented)
method StartContainer() -> (notimplemented: NotImplemented)
method StopContainer(name: string, timeout: int) -> (container: string)
method RestartContainer(name: string, timeout: int) -> (container: string)
method KillContainer(name: string, signal: int) -> (container: string)
method UpdateContainer() -> (notimplemented: NotImplemented)
method RenameContainer() -> (notimplemented: NotImplemented)
method PauseContainer(name: string) -> (container: string)
method UnpauseContainer(name: string) -> (container: string)
method AttachToContainer() -> (notimplemented: NotImplemented)
method WaitContainer(name: string) -> (exitcode: int)
method RemoveContainer(name: string, force: bool) -> (container: string)
method DeleteStoppedContainers() -> (containers: []string)

# Images
method ListImages() -> (images: []ImageInList)
method BuildImage() -> (notimplemented: NotImplemented)
method CreateImage() -> (notimplemented: NotImplemented)
method InspectImage(name: string) -> (image: string)
method HistoryImage(name: string) -> (history: []ImageHistory)
method PushImage(name: string, tag: string, tlsverify: bool) -> ()
method TagImage(name: string, tagged: string) -> ()
method RemoveImage(name: string, force: bool) -> (image: string)
method SearchImage(name: string, limit: int) -> (images: []ImageSearch)
method DeleteUnusedImages() -> (images: []string)
method CreateFromContainer() -> (notimplemented: NotImplemented)
method ImportImage(source: string, reference: string, message: string, changes: []string) -> (image: string)
method ExportImage(name: string, destination: string, compress: bool) -> ()
method PullImage(name: string) -> (id: string)


# Something failed
error ActionFailed (reason: string)
error ImageNotFound (name: string)
error ContainerNotFound (name: string)
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
