// Generated with github.com/varlink/go/cmd/varlink-go-interface-generator
package ioprojectatomicpodman

import "github.com/varlink/go/varlink"

// Type declarations
type ContainerMount struct {
	Destination string   `json:"destination"`
	Type        string   `json:"type"`
	Source      string   `json:"source"`
	Options     []string `json:"options"`
}

type InfoStore struct {
	Containers           int64           `json:"containers"`
	Images               int64           `json:"images"`
	Graph_driver_name    string          `json:"graph_driver_name"`
	Graph_driver_options string          `json:"graph_driver_options"`
	Graph_root           string          `json:"graph_root"`
	Graph_status         InfoGraphStatus `json:"graph_status"`
	Run_root             string          `json:"run_root"`
}

type PodmanInfo struct {
	Host                InfoHost         `json:"host"`
	Registries          []string         `json:"registries"`
	Insecure_registries []string         `json:"insecure_registries"`
	Store               InfoStore        `json:"store"`
	Podman              InfoPodmanBinary `json:"podman"`
}

type Version struct {
	Version    string `json:"version"`
	Go_version string `json:"go_version"`
	Git_commit string `json:"git_commit"`
	Built      int64  `json:"built"`
	Os_arch    string `json:"os_arch"`
}

type StringResponse struct {
	Message string `json:"message"`
}

type ContainerChanges struct {
	Changed []string `json:"changed"`
	Added   []string `json:"added"`
	Deleted []string `json:"deleted"`
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

type InfoPodmanBinary struct {
	Compiler       string `json:"compiler"`
	Go_version     string `json:"go_version"`
	Podman_version string `json:"podman_version"`
	Git_commit     string `json:"git_commit"`
}

type ImageSearch struct {
	Description  string `json:"description"`
	Is_official  bool   `json:"is_official"`
	Is_automated bool   `json:"is_automated"`
	Name         string `json:"name"`
	Star_count   int64  `json:"star_count"`
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

type InfoGraphStatus struct {
	Backing_filesystem  string `json:"backing_filesystem"`
	Native_overlay_diff string `json:"native_overlay_diff"`
	Supports_d_type     string `json:"supports_d_type"`
}

type NotImplemented struct {
	Comment string `json:"comment"`
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

type InfoHost struct {
	Mem_free   int64  `json:"mem_free"`
	Mem_total  int64  `json:"mem_total"`
	Swap_free  int64  `json:"swap_free"`
	Swap_total int64  `json:"swap_total"`
	Arch       string `json:"arch"`
	Cpus       int64  `json:"cpus"`
	Hostname   string `json:"hostname"`
	Kernel     string `json:"kernel"`
	Os         string `json:"os"`
	Uptime     string `json:"uptime"`
}

// Client method calls
type UpdateContainer_methods struct{}

func UpdateContainer() UpdateContainer_methods { return UpdateContainer_methods{} }

func (m UpdateContainer_methods) Call(c *varlink.Connection) (notimplemented_out_ NotImplemented, err_ error) {
	receive, err_ := m.Send(c, 0)
	if err_ != nil {
		return
	}
	notimplemented_out_, _, err_ = receive()
	return
}

func (m UpdateContainer_methods) Send(c *varlink.Connection, flags uint64) (func() (NotImplemented, uint64, error), error) {
	receive, err := c.Send("io.projectatomic.podman.UpdateContainer", nil, flags)
	if err != nil {
		return nil, err
	}
	return func() (notimplemented_out_ NotImplemented, flags uint64, err error) {
		var out struct {
			Notimplemented NotImplemented `json:"notimplemented"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		notimplemented_out_ = out.Notimplemented
		return
	}, nil
}

type InspectImage_methods struct{}

func InspectImage() InspectImage_methods { return InspectImage_methods{} }

func (m InspectImage_methods) Call(c *varlink.Connection, name_in_ string) (image_out_ string, err_ error) {
	receive, err_ := m.Send(c, 0, name_in_)
	if err_ != nil {
		return
	}
	image_out_, _, err_ = receive()
	return
}

func (m InspectImage_methods) Send(c *varlink.Connection, flags uint64, name_in_ string) (func() (string, uint64, error), error) {
	var in struct {
		Name string `json:"name"`
	}
	in.Name = name_in_
	receive, err := c.Send("io.projectatomic.podman.InspectImage", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (image_out_ string, flags uint64, err error) {
		var out struct {
			Image string `json:"image"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		image_out_ = out.Image
		return
	}, nil
}

type GetInfo_methods struct{}

func GetInfo() GetInfo_methods { return GetInfo_methods{} }

func (m GetInfo_methods) Call(c *varlink.Connection) (info_out_ PodmanInfo, err_ error) {
	receive, err_ := m.Send(c, 0)
	if err_ != nil {
		return
	}
	info_out_, _, err_ = receive()
	return
}

func (m GetInfo_methods) Send(c *varlink.Connection, flags uint64) (func() (PodmanInfo, uint64, error), error) {
	receive, err := c.Send("io.projectatomic.podman.GetInfo", nil, flags)
	if err != nil {
		return nil, err
	}
	return func() (info_out_ PodmanInfo, flags uint64, err error) {
		var out struct {
			Info PodmanInfo `json:"info"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		info_out_ = out.Info
		return
	}, nil
}

type ResizeContainerTty_methods struct{}

func ResizeContainerTty() ResizeContainerTty_methods { return ResizeContainerTty_methods{} }

func (m ResizeContainerTty_methods) Call(c *varlink.Connection) (notimplemented_out_ NotImplemented, err_ error) {
	receive, err_ := m.Send(c, 0)
	if err_ != nil {
		return
	}
	notimplemented_out_, _, err_ = receive()
	return
}

func (m ResizeContainerTty_methods) Send(c *varlink.Connection, flags uint64) (func() (NotImplemented, uint64, error), error) {
	receive, err := c.Send("io.projectatomic.podman.ResizeContainerTty", nil, flags)
	if err != nil {
		return nil, err
	}
	return func() (notimplemented_out_ NotImplemented, flags uint64, err error) {
		var out struct {
			Notimplemented NotImplemented `json:"notimplemented"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		notimplemented_out_ = out.Notimplemented
		return
	}, nil
}

type StartContainer_methods struct{}

func StartContainer() StartContainer_methods { return StartContainer_methods{} }

func (m StartContainer_methods) Call(c *varlink.Connection) (notimplemented_out_ NotImplemented, err_ error) {
	receive, err_ := m.Send(c, 0)
	if err_ != nil {
		return
	}
	notimplemented_out_, _, err_ = receive()
	return
}

func (m StartContainer_methods) Send(c *varlink.Connection, flags uint64) (func() (NotImplemented, uint64, error), error) {
	receive, err := c.Send("io.projectatomic.podman.StartContainer", nil, flags)
	if err != nil {
		return nil, err
	}
	return func() (notimplemented_out_ NotImplemented, flags uint64, err error) {
		var out struct {
			Notimplemented NotImplemented `json:"notimplemented"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		notimplemented_out_ = out.Notimplemented
		return
	}, nil
}

type StopContainer_methods struct{}

func StopContainer() StopContainer_methods { return StopContainer_methods{} }

func (m StopContainer_methods) Call(c *varlink.Connection, name_in_ string, timeout_in_ int64) (container_out_ string, err_ error) {
	receive, err_ := m.Send(c, 0, name_in_, timeout_in_)
	if err_ != nil {
		return
	}
	container_out_, _, err_ = receive()
	return
}

func (m StopContainer_methods) Send(c *varlink.Connection, flags uint64, name_in_ string, timeout_in_ int64) (func() (string, uint64, error), error) {
	var in struct {
		Name    string `json:"name"`
		Timeout int64  `json:"timeout"`
	}
	in.Name = name_in_
	in.Timeout = timeout_in_
	receive, err := c.Send("io.projectatomic.podman.StopContainer", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (container_out_ string, flags uint64, err error) {
		var out struct {
			Container string `json:"container"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		container_out_ = out.Container
		return
	}, nil
}

type AttachToContainer_methods struct{}

func AttachToContainer() AttachToContainer_methods { return AttachToContainer_methods{} }

func (m AttachToContainer_methods) Call(c *varlink.Connection) (notimplemented_out_ NotImplemented, err_ error) {
	receive, err_ := m.Send(c, 0)
	if err_ != nil {
		return
	}
	notimplemented_out_, _, err_ = receive()
	return
}

func (m AttachToContainer_methods) Send(c *varlink.Connection, flags uint64) (func() (NotImplemented, uint64, error), error) {
	receive, err := c.Send("io.projectatomic.podman.AttachToContainer", nil, flags)
	if err != nil {
		return nil, err
	}
	return func() (notimplemented_out_ NotImplemented, flags uint64, err error) {
		var out struct {
			Notimplemented NotImplemented `json:"notimplemented"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		notimplemented_out_ = out.Notimplemented
		return
	}, nil
}

type BuildImage_methods struct{}

func BuildImage() BuildImage_methods { return BuildImage_methods{} }

func (m BuildImage_methods) Call(c *varlink.Connection) (notimplemented_out_ NotImplemented, err_ error) {
	receive, err_ := m.Send(c, 0)
	if err_ != nil {
		return
	}
	notimplemented_out_, _, err_ = receive()
	return
}

func (m BuildImage_methods) Send(c *varlink.Connection, flags uint64) (func() (NotImplemented, uint64, error), error) {
	receive, err := c.Send("io.projectatomic.podman.BuildImage", nil, flags)
	if err != nil {
		return nil, err
	}
	return func() (notimplemented_out_ NotImplemented, flags uint64, err error) {
		var out struct {
			Notimplemented NotImplemented `json:"notimplemented"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		notimplemented_out_ = out.Notimplemented
		return
	}, nil
}

type CreateImage_methods struct{}

func CreateImage() CreateImage_methods { return CreateImage_methods{} }

func (m CreateImage_methods) Call(c *varlink.Connection) (notimplemented_out_ NotImplemented, err_ error) {
	receive, err_ := m.Send(c, 0)
	if err_ != nil {
		return
	}
	notimplemented_out_, _, err_ = receive()
	return
}

func (m CreateImage_methods) Send(c *varlink.Connection, flags uint64) (func() (NotImplemented, uint64, error), error) {
	receive, err := c.Send("io.projectatomic.podman.CreateImage", nil, flags)
	if err != nil {
		return nil, err
	}
	return func() (notimplemented_out_ NotImplemented, flags uint64, err error) {
		var out struct {
			Notimplemented NotImplemented `json:"notimplemented"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		notimplemented_out_ = out.Notimplemented
		return
	}, nil
}

type HistoryImage_methods struct{}

func HistoryImage() HistoryImage_methods { return HistoryImage_methods{} }

func (m HistoryImage_methods) Call(c *varlink.Connection, name_in_ string) (history_out_ []ImageHistory, err_ error) {
	receive, err_ := m.Send(c, 0, name_in_)
	if err_ != nil {
		return
	}
	history_out_, _, err_ = receive()
	return
}

func (m HistoryImage_methods) Send(c *varlink.Connection, flags uint64, name_in_ string) (func() ([]ImageHistory, uint64, error), error) {
	var in struct {
		Name string `json:"name"`
	}
	in.Name = name_in_
	receive, err := c.Send("io.projectatomic.podman.HistoryImage", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (history_out_ []ImageHistory, flags uint64, err error) {
		var out struct {
			History []ImageHistory `json:"history"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		history_out_ = []ImageHistory(out.History)
		return
	}, nil
}

type Commit_methods struct{}

func Commit() Commit_methods { return Commit_methods{} }

func (m Commit_methods) Call(c *varlink.Connection, name_in_ string, image_name_in_ string, changes_in_ []string, author_in_ string, message_in_ string, pause_in_ bool) (image_out_ string, err_ error) {
	receive, err_ := m.Send(c, 0, name_in_, image_name_in_, changes_in_, author_in_, message_in_, pause_in_)
	if err_ != nil {
		return
	}
	image_out_, _, err_ = receive()
	return
}

func (m Commit_methods) Send(c *varlink.Connection, flags uint64, name_in_ string, image_name_in_ string, changes_in_ []string, author_in_ string, message_in_ string, pause_in_ bool) (func() (string, uint64, error), error) {
	var in struct {
		Name       string   `json:"name"`
		Image_name string   `json:"image_name"`
		Changes    []string `json:"changes"`
		Author     string   `json:"author"`
		Message    string   `json:"message"`
		Pause      bool     `json:"pause"`
	}
	in.Name = name_in_
	in.Image_name = image_name_in_
	in.Changes = []string(changes_in_)
	in.Author = author_in_
	in.Message = message_in_
	in.Pause = pause_in_
	receive, err := c.Send("io.projectatomic.podman.Commit", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (image_out_ string, flags uint64, err error) {
		var out struct {
			Image string `json:"image"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		image_out_ = out.Image
		return
	}, nil
}

type ListContainers_methods struct{}

func ListContainers() ListContainers_methods { return ListContainers_methods{} }

func (m ListContainers_methods) Call(c *varlink.Connection) (containers_out_ []ListContainerData, err_ error) {
	receive, err_ := m.Send(c, 0)
	if err_ != nil {
		return
	}
	containers_out_, _, err_ = receive()
	return
}

func (m ListContainers_methods) Send(c *varlink.Connection, flags uint64) (func() ([]ListContainerData, uint64, error), error) {
	receive, err := c.Send("io.projectatomic.podman.ListContainers", nil, flags)
	if err != nil {
		return nil, err
	}
	return func() (containers_out_ []ListContainerData, flags uint64, err error) {
		var out struct {
			Containers []ListContainerData `json:"containers"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		containers_out_ = []ListContainerData(out.Containers)
		return
	}, nil
}

type GetContainer_methods struct{}

func GetContainer() GetContainer_methods { return GetContainer_methods{} }

func (m GetContainer_methods) Call(c *varlink.Connection, name_in_ string) (container_out_ ListContainerData, err_ error) {
	receive, err_ := m.Send(c, 0, name_in_)
	if err_ != nil {
		return
	}
	container_out_, _, err_ = receive()
	return
}

func (m GetContainer_methods) Send(c *varlink.Connection, flags uint64, name_in_ string) (func() (ListContainerData, uint64, error), error) {
	var in struct {
		Name string `json:"name"`
	}
	in.Name = name_in_
	receive, err := c.Send("io.projectatomic.podman.GetContainer", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (container_out_ ListContainerData, flags uint64, err error) {
		var out struct {
			Container ListContainerData `json:"container"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		container_out_ = out.Container
		return
	}, nil
}

type ExportContainer_methods struct{}

func ExportContainer() ExportContainer_methods { return ExportContainer_methods{} }

func (m ExportContainer_methods) Call(c *varlink.Connection, name_in_ string, path_in_ string) (tarfile_out_ string, err_ error) {
	receive, err_ := m.Send(c, 0, name_in_, path_in_)
	if err_ != nil {
		return
	}
	tarfile_out_, _, err_ = receive()
	return
}

func (m ExportContainer_methods) Send(c *varlink.Connection, flags uint64, name_in_ string, path_in_ string) (func() (string, uint64, error), error) {
	var in struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	in.Name = name_in_
	in.Path = path_in_
	receive, err := c.Send("io.projectatomic.podman.ExportContainer", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (tarfile_out_ string, flags uint64, err error) {
		var out struct {
			Tarfile string `json:"tarfile"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		tarfile_out_ = out.Tarfile
		return
	}, nil
}

type ListImages_methods struct{}

func ListImages() ListImages_methods { return ListImages_methods{} }

func (m ListImages_methods) Call(c *varlink.Connection) (images_out_ []ImageInList, err_ error) {
	receive, err_ := m.Send(c, 0)
	if err_ != nil {
		return
	}
	images_out_, _, err_ = receive()
	return
}

func (m ListImages_methods) Send(c *varlink.Connection, flags uint64) (func() ([]ImageInList, uint64, error), error) {
	receive, err := c.Send("io.projectatomic.podman.ListImages", nil, flags)
	if err != nil {
		return nil, err
	}
	return func() (images_out_ []ImageInList, flags uint64, err error) {
		var out struct {
			Images []ImageInList `json:"images"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		images_out_ = []ImageInList(out.Images)
		return
	}, nil
}

type DeleteUnusedImages_methods struct{}

func DeleteUnusedImages() DeleteUnusedImages_methods { return DeleteUnusedImages_methods{} }

func (m DeleteUnusedImages_methods) Call(c *varlink.Connection) (images_out_ []string, err_ error) {
	receive, err_ := m.Send(c, 0)
	if err_ != nil {
		return
	}
	images_out_, _, err_ = receive()
	return
}

func (m DeleteUnusedImages_methods) Send(c *varlink.Connection, flags uint64) (func() ([]string, uint64, error), error) {
	receive, err := c.Send("io.projectatomic.podman.DeleteUnusedImages", nil, flags)
	if err != nil {
		return nil, err
	}
	return func() (images_out_ []string, flags uint64, err error) {
		var out struct {
			Images []string `json:"images"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		images_out_ = []string(out.Images)
		return
	}, nil
}

type GetContainerStats_methods struct{}

func GetContainerStats() GetContainerStats_methods { return GetContainerStats_methods{} }

func (m GetContainerStats_methods) Call(c *varlink.Connection, name_in_ string) (container_out_ ContainerStats, err_ error) {
	receive, err_ := m.Send(c, 0, name_in_)
	if err_ != nil {
		return
	}
	container_out_, _, err_ = receive()
	return
}

func (m GetContainerStats_methods) Send(c *varlink.Connection, flags uint64, name_in_ string) (func() (ContainerStats, uint64, error), error) {
	var in struct {
		Name string `json:"name"`
	}
	in.Name = name_in_
	receive, err := c.Send("io.projectatomic.podman.GetContainerStats", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (container_out_ ContainerStats, flags uint64, err error) {
		var out struct {
			Container ContainerStats `json:"container"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		container_out_ = out.Container
		return
	}, nil
}

type UnpauseContainer_methods struct{}

func UnpauseContainer() UnpauseContainer_methods { return UnpauseContainer_methods{} }

func (m UnpauseContainer_methods) Call(c *varlink.Connection, name_in_ string) (container_out_ string, err_ error) {
	receive, err_ := m.Send(c, 0, name_in_)
	if err_ != nil {
		return
	}
	container_out_, _, err_ = receive()
	return
}

func (m UnpauseContainer_methods) Send(c *varlink.Connection, flags uint64, name_in_ string) (func() (string, uint64, error), error) {
	var in struct {
		Name string `json:"name"`
	}
	in.Name = name_in_
	receive, err := c.Send("io.projectatomic.podman.UnpauseContainer", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (container_out_ string, flags uint64, err error) {
		var out struct {
			Container string `json:"container"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		container_out_ = out.Container
		return
	}, nil
}

type PullImage_methods struct{}

func PullImage() PullImage_methods { return PullImage_methods{} }

func (m PullImage_methods) Call(c *varlink.Connection, name_in_ string) (id_out_ string, err_ error) {
	receive, err_ := m.Send(c, 0, name_in_)
	if err_ != nil {
		return
	}
	id_out_, _, err_ = receive()
	return
}

func (m PullImage_methods) Send(c *varlink.Connection, flags uint64, name_in_ string) (func() (string, uint64, error), error) {
	var in struct {
		Name string `json:"name"`
	}
	in.Name = name_in_
	receive, err := c.Send("io.projectatomic.podman.PullImage", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (id_out_ string, flags uint64, err error) {
		var out struct {
			Id string `json:"id"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		id_out_ = out.Id
		return
	}, nil
}

type Ping_methods struct{}

func Ping() Ping_methods { return Ping_methods{} }

func (m Ping_methods) Call(c *varlink.Connection) (ping_out_ StringResponse, err_ error) {
	receive, err_ := m.Send(c, 0)
	if err_ != nil {
		return
	}
	ping_out_, _, err_ = receive()
	return
}

func (m Ping_methods) Send(c *varlink.Connection, flags uint64) (func() (StringResponse, uint64, error), error) {
	receive, err := c.Send("io.projectatomic.podman.Ping", nil, flags)
	if err != nil {
		return nil, err
	}
	return func() (ping_out_ StringResponse, flags uint64, err error) {
		var out struct {
			Ping StringResponse `json:"ping"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		ping_out_ = out.Ping
		return
	}, nil
}

type GetVersion_methods struct{}

func GetVersion() GetVersion_methods { return GetVersion_methods{} }

func (m GetVersion_methods) Call(c *varlink.Connection) (version_out_ Version, err_ error) {
	receive, err_ := m.Send(c, 0)
	if err_ != nil {
		return
	}
	version_out_, _, err_ = receive()
	return
}

func (m GetVersion_methods) Send(c *varlink.Connection, flags uint64) (func() (Version, uint64, error), error) {
	receive, err := c.Send("io.projectatomic.podman.GetVersion", nil, flags)
	if err != nil {
		return nil, err
	}
	return func() (version_out_ Version, flags uint64, err error) {
		var out struct {
			Version Version `json:"version"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		version_out_ = out.Version
		return
	}, nil
}

type CreateContainer_methods struct{}

func CreateContainer() CreateContainer_methods { return CreateContainer_methods{} }

func (m CreateContainer_methods) Call(c *varlink.Connection) (notimplemented_out_ NotImplemented, err_ error) {
	receive, err_ := m.Send(c, 0)
	if err_ != nil {
		return
	}
	notimplemented_out_, _, err_ = receive()
	return
}

func (m CreateContainer_methods) Send(c *varlink.Connection, flags uint64) (func() (NotImplemented, uint64, error), error) {
	receive, err := c.Send("io.projectatomic.podman.CreateContainer", nil, flags)
	if err != nil {
		return nil, err
	}
	return func() (notimplemented_out_ NotImplemented, flags uint64, err error) {
		var out struct {
			Notimplemented NotImplemented `json:"notimplemented"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		notimplemented_out_ = out.Notimplemented
		return
	}, nil
}

type KillContainer_methods struct{}

func KillContainer() KillContainer_methods { return KillContainer_methods{} }

func (m KillContainer_methods) Call(c *varlink.Connection, name_in_ string, signal_in_ int64) (container_out_ string, err_ error) {
	receive, err_ := m.Send(c, 0, name_in_, signal_in_)
	if err_ != nil {
		return
	}
	container_out_, _, err_ = receive()
	return
}

func (m KillContainer_methods) Send(c *varlink.Connection, flags uint64, name_in_ string, signal_in_ int64) (func() (string, uint64, error), error) {
	var in struct {
		Name   string `json:"name"`
		Signal int64  `json:"signal"`
	}
	in.Name = name_in_
	in.Signal = signal_in_
	receive, err := c.Send("io.projectatomic.podman.KillContainer", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (container_out_ string, flags uint64, err error) {
		var out struct {
			Container string `json:"container"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		container_out_ = out.Container
		return
	}, nil
}

type RemoveContainer_methods struct{}

func RemoveContainer() RemoveContainer_methods { return RemoveContainer_methods{} }

func (m RemoveContainer_methods) Call(c *varlink.Connection, name_in_ string, force_in_ bool) (container_out_ string, err_ error) {
	receive, err_ := m.Send(c, 0, name_in_, force_in_)
	if err_ != nil {
		return
	}
	container_out_, _, err_ = receive()
	return
}

func (m RemoveContainer_methods) Send(c *varlink.Connection, flags uint64, name_in_ string, force_in_ bool) (func() (string, uint64, error), error) {
	var in struct {
		Name  string `json:"name"`
		Force bool   `json:"force"`
	}
	in.Name = name_in_
	in.Force = force_in_
	receive, err := c.Send("io.projectatomic.podman.RemoveContainer", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (container_out_ string, flags uint64, err error) {
		var out struct {
			Container string `json:"container"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		container_out_ = out.Container
		return
	}, nil
}

type RemoveImage_methods struct{}

func RemoveImage() RemoveImage_methods { return RemoveImage_methods{} }

func (m RemoveImage_methods) Call(c *varlink.Connection, name_in_ string, force_in_ bool) (image_out_ string, err_ error) {
	receive, err_ := m.Send(c, 0, name_in_, force_in_)
	if err_ != nil {
		return
	}
	image_out_, _, err_ = receive()
	return
}

func (m RemoveImage_methods) Send(c *varlink.Connection, flags uint64, name_in_ string, force_in_ bool) (func() (string, uint64, error), error) {
	var in struct {
		Name  string `json:"name"`
		Force bool   `json:"force"`
	}
	in.Name = name_in_
	in.Force = force_in_
	receive, err := c.Send("io.projectatomic.podman.RemoveImage", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (image_out_ string, flags uint64, err error) {
		var out struct {
			Image string `json:"image"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		image_out_ = out.Image
		return
	}, nil
}

type ImportImage_methods struct{}

func ImportImage() ImportImage_methods { return ImportImage_methods{} }

func (m ImportImage_methods) Call(c *varlink.Connection, source_in_ string, reference_in_ string, message_in_ string, changes_in_ []string) (image_out_ string, err_ error) {
	receive, err_ := m.Send(c, 0, source_in_, reference_in_, message_in_, changes_in_)
	if err_ != nil {
		return
	}
	image_out_, _, err_ = receive()
	return
}

func (m ImportImage_methods) Send(c *varlink.Connection, flags uint64, source_in_ string, reference_in_ string, message_in_ string, changes_in_ []string) (func() (string, uint64, error), error) {
	var in struct {
		Source    string   `json:"source"`
		Reference string   `json:"reference"`
		Message   string   `json:"message"`
		Changes   []string `json:"changes"`
	}
	in.Source = source_in_
	in.Reference = reference_in_
	in.Message = message_in_
	in.Changes = []string(changes_in_)
	receive, err := c.Send("io.projectatomic.podman.ImportImage", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (image_out_ string, flags uint64, err error) {
		var out struct {
			Image string `json:"image"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		image_out_ = out.Image
		return
	}, nil
}

type ExportImage_methods struct{}

func ExportImage() ExportImage_methods { return ExportImage_methods{} }

func (m ExportImage_methods) Call(c *varlink.Connection, name_in_ string, destination_in_ string, compress_in_ bool) (image_out_ string, err_ error) {
	receive, err_ := m.Send(c, 0, name_in_, destination_in_, compress_in_)
	if err_ != nil {
		return
	}
	image_out_, _, err_ = receive()
	return
}

func (m ExportImage_methods) Send(c *varlink.Connection, flags uint64, name_in_ string, destination_in_ string, compress_in_ bool) (func() (string, uint64, error), error) {
	var in struct {
		Name        string `json:"name"`
		Destination string `json:"destination"`
		Compress    bool   `json:"compress"`
	}
	in.Name = name_in_
	in.Destination = destination_in_
	in.Compress = compress_in_
	receive, err := c.Send("io.projectatomic.podman.ExportImage", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (image_out_ string, flags uint64, err error) {
		var out struct {
			Image string `json:"image"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		image_out_ = out.Image
		return
	}, nil
}

type InspectContainer_methods struct{}

func InspectContainer() InspectContainer_methods { return InspectContainer_methods{} }

func (m InspectContainer_methods) Call(c *varlink.Connection, name_in_ string) (container_out_ string, err_ error) {
	receive, err_ := m.Send(c, 0, name_in_)
	if err_ != nil {
		return
	}
	container_out_, _, err_ = receive()
	return
}

func (m InspectContainer_methods) Send(c *varlink.Connection, flags uint64, name_in_ string) (func() (string, uint64, error), error) {
	var in struct {
		Name string `json:"name"`
	}
	in.Name = name_in_
	receive, err := c.Send("io.projectatomic.podman.InspectContainer", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (container_out_ string, flags uint64, err error) {
		var out struct {
			Container string `json:"container"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		container_out_ = out.Container
		return
	}, nil
}

type GetContainerLogs_methods struct{}

func GetContainerLogs() GetContainerLogs_methods { return GetContainerLogs_methods{} }

func (m GetContainerLogs_methods) Call(c *varlink.Connection, name_in_ string) (container_out_ []string, err_ error) {
	receive, err_ := m.Send(c, 0, name_in_)
	if err_ != nil {
		return
	}
	container_out_, _, err_ = receive()
	return
}

func (m GetContainerLogs_methods) Send(c *varlink.Connection, flags uint64, name_in_ string) (func() ([]string, uint64, error), error) {
	var in struct {
		Name string `json:"name"`
	}
	in.Name = name_in_
	receive, err := c.Send("io.projectatomic.podman.GetContainerLogs", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (container_out_ []string, flags uint64, err error) {
		var out struct {
			Container []string `json:"container"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		container_out_ = []string(out.Container)
		return
	}, nil
}

type ListContainerChanges_methods struct{}

func ListContainerChanges() ListContainerChanges_methods { return ListContainerChanges_methods{} }

func (m ListContainerChanges_methods) Call(c *varlink.Connection, name_in_ string) (container_out_ ContainerChanges, err_ error) {
	receive, err_ := m.Send(c, 0, name_in_)
	if err_ != nil {
		return
	}
	container_out_, _, err_ = receive()
	return
}

func (m ListContainerChanges_methods) Send(c *varlink.Connection, flags uint64, name_in_ string) (func() (ContainerChanges, uint64, error), error) {
	var in struct {
		Name string `json:"name"`
	}
	in.Name = name_in_
	receive, err := c.Send("io.projectatomic.podman.ListContainerChanges", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (container_out_ ContainerChanges, flags uint64, err error) {
		var out struct {
			Container ContainerChanges `json:"container"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		container_out_ = out.Container
		return
	}, nil
}

type RestartContainer_methods struct{}

func RestartContainer() RestartContainer_methods { return RestartContainer_methods{} }

func (m RestartContainer_methods) Call(c *varlink.Connection, name_in_ string, timeout_in_ int64) (container_out_ string, err_ error) {
	receive, err_ := m.Send(c, 0, name_in_, timeout_in_)
	if err_ != nil {
		return
	}
	container_out_, _, err_ = receive()
	return
}

func (m RestartContainer_methods) Send(c *varlink.Connection, flags uint64, name_in_ string, timeout_in_ int64) (func() (string, uint64, error), error) {
	var in struct {
		Name    string `json:"name"`
		Timeout int64  `json:"timeout"`
	}
	in.Name = name_in_
	in.Timeout = timeout_in_
	receive, err := c.Send("io.projectatomic.podman.RestartContainer", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (container_out_ string, flags uint64, err error) {
		var out struct {
			Container string `json:"container"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		container_out_ = out.Container
		return
	}, nil
}

type RenameContainer_methods struct{}

func RenameContainer() RenameContainer_methods { return RenameContainer_methods{} }

func (m RenameContainer_methods) Call(c *varlink.Connection) (notimplemented_out_ NotImplemented, err_ error) {
	receive, err_ := m.Send(c, 0)
	if err_ != nil {
		return
	}
	notimplemented_out_, _, err_ = receive()
	return
}

func (m RenameContainer_methods) Send(c *varlink.Connection, flags uint64) (func() (NotImplemented, uint64, error), error) {
	receive, err := c.Send("io.projectatomic.podman.RenameContainer", nil, flags)
	if err != nil {
		return nil, err
	}
	return func() (notimplemented_out_ NotImplemented, flags uint64, err error) {
		var out struct {
			Notimplemented NotImplemented `json:"notimplemented"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		notimplemented_out_ = out.Notimplemented
		return
	}, nil
}

type PauseContainer_methods struct{}

func PauseContainer() PauseContainer_methods { return PauseContainer_methods{} }

func (m PauseContainer_methods) Call(c *varlink.Connection, name_in_ string) (container_out_ string, err_ error) {
	receive, err_ := m.Send(c, 0, name_in_)
	if err_ != nil {
		return
	}
	container_out_, _, err_ = receive()
	return
}

func (m PauseContainer_methods) Send(c *varlink.Connection, flags uint64, name_in_ string) (func() (string, uint64, error), error) {
	var in struct {
		Name string `json:"name"`
	}
	in.Name = name_in_
	receive, err := c.Send("io.projectatomic.podman.PauseContainer", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (container_out_ string, flags uint64, err error) {
		var out struct {
			Container string `json:"container"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		container_out_ = out.Container
		return
	}, nil
}

type PushImage_methods struct{}

func PushImage() PushImage_methods { return PushImage_methods{} }

func (m PushImage_methods) Call(c *varlink.Connection, name_in_ string, tag_in_ string, tlsverify_in_ bool) (image_out_ string, err_ error) {
	receive, err_ := m.Send(c, 0, name_in_, tag_in_, tlsverify_in_)
	if err_ != nil {
		return
	}
	image_out_, _, err_ = receive()
	return
}

func (m PushImage_methods) Send(c *varlink.Connection, flags uint64, name_in_ string, tag_in_ string, tlsverify_in_ bool) (func() (string, uint64, error), error) {
	var in struct {
		Name      string `json:"name"`
		Tag       string `json:"tag"`
		Tlsverify bool   `json:"tlsverify"`
	}
	in.Name = name_in_
	in.Tag = tag_in_
	in.Tlsverify = tlsverify_in_
	receive, err := c.Send("io.projectatomic.podman.PushImage", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (image_out_ string, flags uint64, err error) {
		var out struct {
			Image string `json:"image"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		image_out_ = out.Image
		return
	}, nil
}

type ListContainerProcesses_methods struct{}

func ListContainerProcesses() ListContainerProcesses_methods { return ListContainerProcesses_methods{} }

func (m ListContainerProcesses_methods) Call(c *varlink.Connection, name_in_ string, opts_in_ []string) (container_out_ []string, err_ error) {
	receive, err_ := m.Send(c, 0, name_in_, opts_in_)
	if err_ != nil {
		return
	}
	container_out_, _, err_ = receive()
	return
}

func (m ListContainerProcesses_methods) Send(c *varlink.Connection, flags uint64, name_in_ string, opts_in_ []string) (func() ([]string, uint64, error), error) {
	var in struct {
		Name string   `json:"name"`
		Opts []string `json:"opts"`
	}
	in.Name = name_in_
	in.Opts = []string(opts_in_)
	receive, err := c.Send("io.projectatomic.podman.ListContainerProcesses", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (container_out_ []string, flags uint64, err error) {
		var out struct {
			Container []string `json:"container"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		container_out_ = []string(out.Container)
		return
	}, nil
}

type WaitContainer_methods struct{}

func WaitContainer() WaitContainer_methods { return WaitContainer_methods{} }

func (m WaitContainer_methods) Call(c *varlink.Connection, name_in_ string) (exitcode_out_ int64, err_ error) {
	receive, err_ := m.Send(c, 0, name_in_)
	if err_ != nil {
		return
	}
	exitcode_out_, _, err_ = receive()
	return
}

func (m WaitContainer_methods) Send(c *varlink.Connection, flags uint64, name_in_ string) (func() (int64, uint64, error), error) {
	var in struct {
		Name string `json:"name"`
	}
	in.Name = name_in_
	receive, err := c.Send("io.projectatomic.podman.WaitContainer", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (exitcode_out_ int64, flags uint64, err error) {
		var out struct {
			Exitcode int64 `json:"exitcode"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		exitcode_out_ = out.Exitcode
		return
	}, nil
}

type DeleteStoppedContainers_methods struct{}

func DeleteStoppedContainers() DeleteStoppedContainers_methods {
	return DeleteStoppedContainers_methods{}
}

func (m DeleteStoppedContainers_methods) Call(c *varlink.Connection) (containers_out_ []string, err_ error) {
	receive, err_ := m.Send(c, 0)
	if err_ != nil {
		return
	}
	containers_out_, _, err_ = receive()
	return
}

func (m DeleteStoppedContainers_methods) Send(c *varlink.Connection, flags uint64) (func() ([]string, uint64, error), error) {
	receive, err := c.Send("io.projectatomic.podman.DeleteStoppedContainers", nil, flags)
	if err != nil {
		return nil, err
	}
	return func() (containers_out_ []string, flags uint64, err error) {
		var out struct {
			Containers []string `json:"containers"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		containers_out_ = []string(out.Containers)
		return
	}, nil
}

type TagImage_methods struct{}

func TagImage() TagImage_methods { return TagImage_methods{} }

func (m TagImage_methods) Call(c *varlink.Connection, name_in_ string, tagged_in_ string) (image_out_ string, err_ error) {
	receive, err_ := m.Send(c, 0, name_in_, tagged_in_)
	if err_ != nil {
		return
	}
	image_out_, _, err_ = receive()
	return
}

func (m TagImage_methods) Send(c *varlink.Connection, flags uint64, name_in_ string, tagged_in_ string) (func() (string, uint64, error), error) {
	var in struct {
		Name   string `json:"name"`
		Tagged string `json:"tagged"`
	}
	in.Name = name_in_
	in.Tagged = tagged_in_
	receive, err := c.Send("io.projectatomic.podman.TagImage", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (image_out_ string, flags uint64, err error) {
		var out struct {
			Image string `json:"image"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		image_out_ = out.Image
		return
	}, nil
}

type SearchImage_methods struct{}

func SearchImage() SearchImage_methods { return SearchImage_methods{} }

func (m SearchImage_methods) Call(c *varlink.Connection, name_in_ string, limit_in_ int64) (images_out_ []ImageSearch, err_ error) {
	receive, err_ := m.Send(c, 0, name_in_, limit_in_)
	if err_ != nil {
		return
	}
	images_out_, _, err_ = receive()
	return
}

func (m SearchImage_methods) Send(c *varlink.Connection, flags uint64, name_in_ string, limit_in_ int64) (func() ([]ImageSearch, uint64, error), error) {
	var in struct {
		Name  string `json:"name"`
		Limit int64  `json:"limit"`
	}
	in.Name = name_in_
	in.Limit = limit_in_
	receive, err := c.Send("io.projectatomic.podman.SearchImage", in, flags)
	if err != nil {
		return nil, err
	}
	return func() (images_out_ []ImageSearch, flags uint64, err error) {
		var out struct {
			Images []ImageSearch `json:"images"`
		}
		flags, err = receive(&out)
		if err != nil {
			return
		}
		images_out_ = []ImageSearch(out.Images)
		return
	}, nil
}

// Service interface with all methods
type ioprojectatomicpodmanInterface interface {
	GetContainerStats(c VarlinkCall, name_ string) error
	UnpauseContainer(c VarlinkCall, name_ string) error
	PullImage(c VarlinkCall, name_ string) error
	ExportImage(c VarlinkCall, name_ string, destination_ string, compress_ bool) error
	Ping(c VarlinkCall) error
	GetVersion(c VarlinkCall) error
	CreateContainer(c VarlinkCall) error
	KillContainer(c VarlinkCall, name_ string, signal_ int64) error
	RemoveContainer(c VarlinkCall, name_ string, force_ bool) error
	RemoveImage(c VarlinkCall, name_ string, force_ bool) error
	ImportImage(c VarlinkCall, source_ string, reference_ string, message_ string, changes_ []string) error
	InspectContainer(c VarlinkCall, name_ string) error
	GetContainerLogs(c VarlinkCall, name_ string) error
	ListContainerChanges(c VarlinkCall, name_ string) error
	RestartContainer(c VarlinkCall, name_ string, timeout_ int64) error
	RenameContainer(c VarlinkCall) error
	PauseContainer(c VarlinkCall, name_ string) error
	PushImage(c VarlinkCall, name_ string, tag_ string, tlsverify_ bool) error
	ListContainerProcesses(c VarlinkCall, name_ string, opts_ []string) error
	WaitContainer(c VarlinkCall, name_ string) error
	DeleteStoppedContainers(c VarlinkCall) error
	TagImage(c VarlinkCall, name_ string, tagged_ string) error
	SearchImage(c VarlinkCall, name_ string, limit_ int64) error
	UpdateContainer(c VarlinkCall) error
	InspectImage(c VarlinkCall, name_ string) error
	HistoryImage(c VarlinkCall, name_ string) error
	GetInfo(c VarlinkCall) error
	ResizeContainerTty(c VarlinkCall) error
	StartContainer(c VarlinkCall) error
	StopContainer(c VarlinkCall, name_ string, timeout_ int64) error
	AttachToContainer(c VarlinkCall) error
	BuildImage(c VarlinkCall) error
	CreateImage(c VarlinkCall) error
	Commit(c VarlinkCall, name_ string, image_name_ string, changes_ []string, author_ string, message_ string, pause_ bool) error
	ListContainers(c VarlinkCall) error
	GetContainer(c VarlinkCall, name_ string) error
	ExportContainer(c VarlinkCall, name_ string, path_ string) error
	ListImages(c VarlinkCall) error
	DeleteUnusedImages(c VarlinkCall) error
}

// Service object with all methods
type VarlinkCall struct{ varlink.Call }

// Reply methods for all varlink errors
func (c *VarlinkCall) ReplyContainerNotFound(name_ string) error {
	var out struct {
		Name string `json:"name"`
	}
	out.Name = name_
	return c.ReplyError("io.projectatomic.podman.ContainerNotFound", &out)
}

func (c *VarlinkCall) ReplyErrorOccurred(reason_ string) error {
	var out struct {
		Reason string `json:"reason"`
	}
	out.Reason = reason_
	return c.ReplyError("io.projectatomic.podman.ErrorOccurred", &out)
}

func (c *VarlinkCall) ReplyRuntimeError(reason_ string) error {
	var out struct {
		Reason string `json:"reason"`
	}
	out.Reason = reason_
	return c.ReplyError("io.projectatomic.podman.RuntimeError", &out)
}

func (c *VarlinkCall) ReplyImageNotFound(name_ string) error {
	var out struct {
		Name string `json:"name"`
	}
	out.Name = name_
	return c.ReplyError("io.projectatomic.podman.ImageNotFound", &out)
}

// Reply methods for all varlink methods
func (c *VarlinkCall) ReplyListContainerProcesses(container_ []string) error {
	var out struct {
		Container []string `json:"container"`
	}
	out.Container = []string(container_)
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyWaitContainer(exitcode_ int64) error {
	var out struct {
		Exitcode int64 `json:"exitcode"`
	}
	out.Exitcode = exitcode_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyDeleteStoppedContainers(containers_ []string) error {
	var out struct {
		Containers []string `json:"containers"`
	}
	out.Containers = []string(containers_)
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyTagImage(image_ string) error {
	var out struct {
		Image string `json:"image"`
	}
	out.Image = image_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplySearchImage(images_ []ImageSearch) error {
	var out struct {
		Images []ImageSearch `json:"images"`
	}
	out.Images = []ImageSearch(images_)
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyUpdateContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyInspectImage(image_ string) error {
	var out struct {
		Image string `json:"image"`
	}
	out.Image = image_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyAttachToContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyBuildImage(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyCreateImage(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyHistoryImage(history_ []ImageHistory) error {
	var out struct {
		History []ImageHistory `json:"history"`
	}
	out.History = []ImageHistory(history_)
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyGetInfo(info_ PodmanInfo) error {
	var out struct {
		Info PodmanInfo `json:"info"`
	}
	out.Info = info_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyResizeContainerTty(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyStartContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyStopContainer(container_ string) error {
	var out struct {
		Container string `json:"container"`
	}
	out.Container = container_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyCommit(image_ string) error {
	var out struct {
		Image string `json:"image"`
	}
	out.Image = image_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyDeleteUnusedImages(images_ []string) error {
	var out struct {
		Images []string `json:"images"`
	}
	out.Images = []string(images_)
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyListContainers(containers_ []ListContainerData) error {
	var out struct {
		Containers []ListContainerData `json:"containers"`
	}
	out.Containers = []ListContainerData(containers_)
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyGetContainer(container_ ListContainerData) error {
	var out struct {
		Container ListContainerData `json:"container"`
	}
	out.Container = container_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyExportContainer(tarfile_ string) error {
	var out struct {
		Tarfile string `json:"tarfile"`
	}
	out.Tarfile = tarfile_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyListImages(images_ []ImageInList) error {
	var out struct {
		Images []ImageInList `json:"images"`
	}
	out.Images = []ImageInList(images_)
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyGetContainerStats(container_ ContainerStats) error {
	var out struct {
		Container ContainerStats `json:"container"`
	}
	out.Container = container_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyUnpauseContainer(container_ string) error {
	var out struct {
		Container string `json:"container"`
	}
	out.Container = container_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyPullImage(id_ string) error {
	var out struct {
		Id string `json:"id"`
	}
	out.Id = id_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyRemoveContainer(container_ string) error {
	var out struct {
		Container string `json:"container"`
	}
	out.Container = container_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyRemoveImage(image_ string) error {
	var out struct {
		Image string `json:"image"`
	}
	out.Image = image_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyImportImage(image_ string) error {
	var out struct {
		Image string `json:"image"`
	}
	out.Image = image_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyExportImage(image_ string) error {
	var out struct {
		Image string `json:"image"`
	}
	out.Image = image_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyPing(ping_ StringResponse) error {
	var out struct {
		Ping StringResponse `json:"ping"`
	}
	out.Ping = ping_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyGetVersion(version_ Version) error {
	var out struct {
		Version Version `json:"version"`
	}
	out.Version = version_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyCreateContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyKillContainer(container_ string) error {
	var out struct {
		Container string `json:"container"`
	}
	out.Container = container_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyRenameContainer(notimplemented_ NotImplemented) error {
	var out struct {
		Notimplemented NotImplemented `json:"notimplemented"`
	}
	out.Notimplemented = notimplemented_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyPauseContainer(container_ string) error {
	var out struct {
		Container string `json:"container"`
	}
	out.Container = container_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyPushImage(image_ string) error {
	var out struct {
		Image string `json:"image"`
	}
	out.Image = image_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyInspectContainer(container_ string) error {
	var out struct {
		Container string `json:"container"`
	}
	out.Container = container_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyGetContainerLogs(container_ []string) error {
	var out struct {
		Container []string `json:"container"`
	}
	out.Container = []string(container_)
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyListContainerChanges(container_ ContainerChanges) error {
	var out struct {
		Container ContainerChanges `json:"container"`
	}
	out.Container = container_
	return c.Reply(&out)
}

func (c *VarlinkCall) ReplyRestartContainer(container_ string) error {
	var out struct {
		Container string `json:"container"`
	}
	out.Container = container_
	return c.Reply(&out)
}

// Dummy implementations for all varlink methods
func (s *VarlinkInterface) UpdateContainer(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.UpdateContainer")
}

func (s *VarlinkInterface) InspectImage(c VarlinkCall, name_ string) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.InspectImage")
}

func (s *VarlinkInterface) GetInfo(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.GetInfo")
}

func (s *VarlinkInterface) ResizeContainerTty(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.ResizeContainerTty")
}

func (s *VarlinkInterface) StartContainer(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.StartContainer")
}

func (s *VarlinkInterface) StopContainer(c VarlinkCall, name_ string, timeout_ int64) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.StopContainer")
}

func (s *VarlinkInterface) AttachToContainer(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.AttachToContainer")
}

func (s *VarlinkInterface) BuildImage(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.BuildImage")
}

func (s *VarlinkInterface) CreateImage(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.CreateImage")
}

func (s *VarlinkInterface) HistoryImage(c VarlinkCall, name_ string) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.HistoryImage")
}

func (s *VarlinkInterface) Commit(c VarlinkCall, name_ string, image_name_ string, changes_ []string, author_ string, message_ string, pause_ bool) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.Commit")
}

func (s *VarlinkInterface) ListContainers(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.ListContainers")
}

func (s *VarlinkInterface) GetContainer(c VarlinkCall, name_ string) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.GetContainer")
}

func (s *VarlinkInterface) ExportContainer(c VarlinkCall, name_ string, path_ string) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.ExportContainer")
}

func (s *VarlinkInterface) ListImages(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.ListImages")
}

func (s *VarlinkInterface) DeleteUnusedImages(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.DeleteUnusedImages")
}

func (s *VarlinkInterface) GetContainerStats(c VarlinkCall, name_ string) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.GetContainerStats")
}

func (s *VarlinkInterface) UnpauseContainer(c VarlinkCall, name_ string) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.UnpauseContainer")
}

func (s *VarlinkInterface) PullImage(c VarlinkCall, name_ string) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.PullImage")
}

func (s *VarlinkInterface) Ping(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.Ping")
}

func (s *VarlinkInterface) GetVersion(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.GetVersion")
}

func (s *VarlinkInterface) CreateContainer(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.CreateContainer")
}

func (s *VarlinkInterface) KillContainer(c VarlinkCall, name_ string, signal_ int64) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.KillContainer")
}

func (s *VarlinkInterface) RemoveContainer(c VarlinkCall, name_ string, force_ bool) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.RemoveContainer")
}

func (s *VarlinkInterface) RemoveImage(c VarlinkCall, name_ string, force_ bool) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.RemoveImage")
}

func (s *VarlinkInterface) ImportImage(c VarlinkCall, source_ string, reference_ string, message_ string, changes_ []string) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.ImportImage")
}

func (s *VarlinkInterface) ExportImage(c VarlinkCall, name_ string, destination_ string, compress_ bool) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.ExportImage")
}

func (s *VarlinkInterface) InspectContainer(c VarlinkCall, name_ string) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.InspectContainer")
}

func (s *VarlinkInterface) GetContainerLogs(c VarlinkCall, name_ string) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.GetContainerLogs")
}

func (s *VarlinkInterface) ListContainerChanges(c VarlinkCall, name_ string) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.ListContainerChanges")
}

func (s *VarlinkInterface) RestartContainer(c VarlinkCall, name_ string, timeout_ int64) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.RestartContainer")
}

func (s *VarlinkInterface) RenameContainer(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.RenameContainer")
}

func (s *VarlinkInterface) PauseContainer(c VarlinkCall, name_ string) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.PauseContainer")
}

func (s *VarlinkInterface) PushImage(c VarlinkCall, name_ string, tag_ string, tlsverify_ bool) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.PushImage")
}

func (s *VarlinkInterface) ListContainerProcesses(c VarlinkCall, name_ string, opts_ []string) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.ListContainerProcesses")
}

func (s *VarlinkInterface) WaitContainer(c VarlinkCall, name_ string) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.WaitContainer")
}

func (s *VarlinkInterface) DeleteStoppedContainers(c VarlinkCall) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.DeleteStoppedContainers")
}

func (s *VarlinkInterface) TagImage(c VarlinkCall, name_ string, tagged_ string) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.TagImage")
}

func (s *VarlinkInterface) SearchImage(c VarlinkCall, name_ string, limit_ int64) error {
	return c.ReplyMethodNotImplemented("io.projectatomic.podman.SearchImage")
}

// Method call dispatcher
func (s *VarlinkInterface) VarlinkDispatch(call varlink.Call, methodname string) error {
	switch methodname {
	case "GetVersion":
		return s.ioprojectatomicpodmanInterface.GetVersion(VarlinkCall{call})

	case "CreateContainer":
		return s.ioprojectatomicpodmanInterface.CreateContainer(VarlinkCall{call})

	case "KillContainer":
		var in struct {
			Name   string `json:"name"`
			Signal int64  `json:"signal"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s.ioprojectatomicpodmanInterface.KillContainer(VarlinkCall{call}, in.Name, in.Signal)

	case "RemoveContainer":
		var in struct {
			Name  string `json:"name"`
			Force bool   `json:"force"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s.ioprojectatomicpodmanInterface.RemoveContainer(VarlinkCall{call}, in.Name, in.Force)

	case "RemoveImage":
		var in struct {
			Name  string `json:"name"`
			Force bool   `json:"force"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s.ioprojectatomicpodmanInterface.RemoveImage(VarlinkCall{call}, in.Name, in.Force)

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
		return s.ioprojectatomicpodmanInterface.ImportImage(VarlinkCall{call}, in.Source, in.Reference, in.Message, []string(in.Changes))

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
		return s.ioprojectatomicpodmanInterface.ExportImage(VarlinkCall{call}, in.Name, in.Destination, in.Compress)

	case "Ping":
		return s.ioprojectatomicpodmanInterface.Ping(VarlinkCall{call})

	case "GetContainerLogs":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s.ioprojectatomicpodmanInterface.GetContainerLogs(VarlinkCall{call}, in.Name)

	case "ListContainerChanges":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s.ioprojectatomicpodmanInterface.ListContainerChanges(VarlinkCall{call}, in.Name)

	case "RestartContainer":
		var in struct {
			Name    string `json:"name"`
			Timeout int64  `json:"timeout"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s.ioprojectatomicpodmanInterface.RestartContainer(VarlinkCall{call}, in.Name, in.Timeout)

	case "RenameContainer":
		return s.ioprojectatomicpodmanInterface.RenameContainer(VarlinkCall{call})

	case "PauseContainer":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s.ioprojectatomicpodmanInterface.PauseContainer(VarlinkCall{call}, in.Name)

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
		return s.ioprojectatomicpodmanInterface.PushImage(VarlinkCall{call}, in.Name, in.Tag, in.Tlsverify)

	case "InspectContainer":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s.ioprojectatomicpodmanInterface.InspectContainer(VarlinkCall{call}, in.Name)

	case "WaitContainer":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s.ioprojectatomicpodmanInterface.WaitContainer(VarlinkCall{call}, in.Name)

	case "DeleteStoppedContainers":
		return s.ioprojectatomicpodmanInterface.DeleteStoppedContainers(VarlinkCall{call})

	case "TagImage":
		var in struct {
			Name   string `json:"name"`
			Tagged string `json:"tagged"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s.ioprojectatomicpodmanInterface.TagImage(VarlinkCall{call}, in.Name, in.Tagged)

	case "ListContainerProcesses":
		var in struct {
			Name string   `json:"name"`
			Opts []string `json:"opts"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s.ioprojectatomicpodmanInterface.ListContainerProcesses(VarlinkCall{call}, in.Name, []string(in.Opts))

	case "SearchImage":
		var in struct {
			Name  string `json:"name"`
			Limit int64  `json:"limit"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s.ioprojectatomicpodmanInterface.SearchImage(VarlinkCall{call}, in.Name, in.Limit)

	case "InspectImage":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s.ioprojectatomicpodmanInterface.InspectImage(VarlinkCall{call}, in.Name)

	case "UpdateContainer":
		return s.ioprojectatomicpodmanInterface.UpdateContainer(VarlinkCall{call})

	case "ResizeContainerTty":
		return s.ioprojectatomicpodmanInterface.ResizeContainerTty(VarlinkCall{call})

	case "StartContainer":
		return s.ioprojectatomicpodmanInterface.StartContainer(VarlinkCall{call})

	case "StopContainer":
		var in struct {
			Name    string `json:"name"`
			Timeout int64  `json:"timeout"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s.ioprojectatomicpodmanInterface.StopContainer(VarlinkCall{call}, in.Name, in.Timeout)

	case "AttachToContainer":
		return s.ioprojectatomicpodmanInterface.AttachToContainer(VarlinkCall{call})

	case "BuildImage":
		return s.ioprojectatomicpodmanInterface.BuildImage(VarlinkCall{call})

	case "CreateImage":
		return s.ioprojectatomicpodmanInterface.CreateImage(VarlinkCall{call})

	case "HistoryImage":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s.ioprojectatomicpodmanInterface.HistoryImage(VarlinkCall{call}, in.Name)

	case "GetInfo":
		return s.ioprojectatomicpodmanInterface.GetInfo(VarlinkCall{call})

	case "Commit":
		var in struct {
			Name       string   `json:"name"`
			Image_name string   `json:"image_name"`
			Changes    []string `json:"changes"`
			Author     string   `json:"author"`
			Message    string   `json:"message"`
			Pause      bool     `json:"pause"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s.ioprojectatomicpodmanInterface.Commit(VarlinkCall{call}, in.Name, in.Image_name, []string(in.Changes), in.Author, in.Message, in.Pause)

	case "GetContainer":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s.ioprojectatomicpodmanInterface.GetContainer(VarlinkCall{call}, in.Name)

	case "ExportContainer":
		var in struct {
			Name string `json:"name"`
			Path string `json:"path"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s.ioprojectatomicpodmanInterface.ExportContainer(VarlinkCall{call}, in.Name, in.Path)

	case "ListImages":
		return s.ioprojectatomicpodmanInterface.ListImages(VarlinkCall{call})

	case "DeleteUnusedImages":
		return s.ioprojectatomicpodmanInterface.DeleteUnusedImages(VarlinkCall{call})

	case "ListContainers":
		return s.ioprojectatomicpodmanInterface.ListContainers(VarlinkCall{call})

	case "UnpauseContainer":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s.ioprojectatomicpodmanInterface.UnpauseContainer(VarlinkCall{call}, in.Name)

	case "PullImage":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s.ioprojectatomicpodmanInterface.PullImage(VarlinkCall{call}, in.Name)

	case "GetContainerStats":
		var in struct {
			Name string `json:"name"`
		}
		err := call.GetParameters(&in)
		if err != nil {
			return call.ReplyInvalidParameter("parameters")
		}
		return s.ioprojectatomicpodmanInterface.GetContainerStats(VarlinkCall{call}, in.Name)

	default:
		return call.ReplyMethodNotFound(methodname)
	}
}

// Varlink interface name
func (s *VarlinkInterface) VarlinkGetName() string {
	return `io.projectatomic.podman`
}

// Varlink interface description
func (s *VarlinkInterface) VarlinkGetDescription() string {
	return `# Podman Service Interface and API description.  The master version of this document can be found
# in the [API.md](https://github.com/projectatomic/libpod/blob/master/API.md) file in the upstream libpod repository.
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
# ContainerChanges describes the return struct for ListContainerChanges
type ContainerChanges (
   changed: []string,
   added: []string,
   deleted: []string
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

# InfoHost describes the host stats portion of PodmanInfo
type InfoHost (
    mem_free: int,
    mem_total: int,
    swap_free: int,
    swap_total: int,
    arch: string,
    cpus: int,
    hostname: string,
    kernel: string,
    os: string,
    uptime: string
)

# InfoGraphStatus describes the detailed status of the storage driver
type InfoGraphStatus (
    backing_filesystem: string,
    native_overlay_diff: string,
    supports_d_type: string
)

# InfoStore describes the host's storage informatoin
type InfoStore (
    containers: int,
    images: int,
    graph_driver_name: string,
    graph_driver_options: string,
    graph_root: string,
    graph_status: InfoGraphStatus,
    run_root: string
)

# InfoPodman provides details on the podman binary
type InfoPodmanBinary (
    compiler: string,
    go_version: string,
    podman_version: string,
    git_commit: string
)

# PodmanInfo describes the Podman host and build
type PodmanInfo (
    host: InfoHost,
    registries: []string,
    insecure_registries: []string,
    store: InfoStore,
    podman: InfoPodmanBinary
)

# Ping provides a response for developers to ensure their varlink setup is working.
# #### Example
# ~~~
# $ varlink call -m unix:/run/podman/io.projectatomic.podman/io.projectatomic.podman.Ping
# {
#   "ping": {
#     "message": "OK"
#   }
# }
# ~~~
method Ping() -> (ping: StringResponse)

# GetVersion returns a Version structure describing the libpod setup on their
# system.
method GetVersion() -> (version: Version)

# GetInfo returns a [PodmanInfo](#PodmanInfo) struct that describes podman and its host such as storage stats,
# build information of Podman, and system-wide registries.
method GetInfo() -> (info: PodmanInfo)

# ListContainers returns a list of containers in no particular order.  There are
# returned as an array of ListContainerData structs.  See also [GetContainer](#GetContainer).
method ListContainers() -> (containers: []ListContainerData)

# GetContainer takes a name or ID of a container and returns single ListContainerData
# structure.  A [ContainerNotFound](#ContainerNotFound) error will be returned if the container cannot be found.
# See also [ListContainers](ListContainers) and [InspectContainer](InspectContainer).
method GetContainer(name: string) -> (container: ListContainerData)

# This method is not implemented yet.
method CreateContainer() -> (notimplemented: NotImplemented)

# InspectContainer data takes a name or ID of a container returns the inspection
# data in string format.  You can then serialize the string into JSON.  A [ContainerNotFound](#ContainerNotFound)
# error will be returned if the container cannot be found. See also [InspectImage](#InspectImage).
method InspectContainer(name: string) -> (container: string)

# ListContainerProcesses takes a name or ID of a container and returns the processes
# running inside the container as array of strings.  It will accept an array of string
# arguements that represent ps options.  If the container cannot be found, a [ContainerNotFound](#ContainerNotFound)
# error will be returned.
# #### Example
# ~~~
# $ varlink call -m unix:/run/podman/io.projectatomic.podman/io.projectatomic.podman.ListContainerProcesses '{"name": "135d71b9495f", "opts": []}'
# {
#   "container": [
#     "  UID   PID  PPID  C STIME TTY          TIME CMD",
#     "    0 21220 21210  0 09:05 pts/0    00:00:00 /bin/sh",
#     "    0 21232 21220  0 09:05 pts/0    00:00:00 top",
#     "    0 21284 21220  0 09:05 pts/0    00:00:00 vi /etc/hosts"
#   ]
# }
# ~~~
method ListContainerProcesses(name: string, opts: []string) -> (container: []string)

# GetContainerLogs takes a name or ID of a container and returns the logs of that container.
# If the container cannot be found, a [ContainerNotFound](#ContainerNotFound) error will be returned.
# The container logs are returned as an array of strings.  GetContainerLogs will honor the streaming
# capability of varlink if the client invokes it.
method GetContainerLogs(name: string) -> (container: []string)

# ListContainerChanges takes a name or ID of a container and returns changes between the container and
# its base image. It returns a struct of changed, deleted, and added path names. If the
# container cannot be found, a [ContainerNotFound](#ContainerNotFound) error will be returned.
method ListContainerChanges(name: string) -> (container: ContainerChanges)

# ExportContainer creates an image from a container.  It takes the name or ID of a container and a
# path representing the target tarfile.  If the container cannot be found, a [ContainerNotFound](#ContainerNotFound)
# error will be returned.
# The return value is the written tarfile.
method ExportContainer(name: string, path: string) -> (tarfile: string)

# GetContainerStats takes the name or ID of a container and returns a single ContainerStats structure which
# contains attributes like memory and cpu usage.  If the container cannot be found, a
# [ContainerNotFound](#ContainerNotFound)  error will be returned.
# #### Example
# ~~~
# $ varlink call -m unix:/run/podman/io.projectatomic.podman/io.projectatomic.podman.GetContainerStats '{"name": "c33e4164f384"}'
# {
#   "container": {
#     "block_input": 0,
#     "block_output": 0,
#     "cpu": 2.571123918839990154678e-08,
#     "cpu_nano": 49037378,
#     "id": "c33e4164f384aa9d979072a63319d66b74fd7a128be71fa68ede24f33ec6cfee",
#     "mem_limit": 33080606720,
#     "mem_perc": 2.166828456524753747370e-03,
#     "mem_usage": 716800,
#     "name": "competent_wozniak",
#     "net_input": 768,
#     "net_output": 5910,
#     "pids": 1,
#     "system_nano": 10000000
#   }
# }
# ~~~
method GetContainerStats(name: string) -> (container: ContainerStats)

# This method has not be implemented yet.
method ResizeContainerTty() -> (notimplemented: NotImplemented)

# This method has not be implemented yet.
method StartContainer() -> (notimplemented: NotImplemented)

# StopContainer stops a container given a timeout.  It takes the name or ID of a container as well as a
# timeout value.  The timeout value the time before a forceable stop to the container is applied.  It
# returns the container ID once stopped. If the container cannot be found, a [ContainerNotFound](#ContainerNotFound)
# error will be returned instead. See also [KillContainer](KillContainer).
# #### Error
# ~~~
# $ varlink call -m unix:/run/podman/io.projectatomic.podman/io.projectatomic.podman.StopContainer '{"name": "135d71b9495f", "timeout": 5}'
# {
#   "container": "135d71b9495f7c3967f536edad57750bfdb569336cd107d8aabab45565ffcfb6"
# }
# ~~~
method StopContainer(name: string, timeout: int) -> (container: string)

# RestartContainer will restart a running container given a container name or ID and timeout value. The timeout
# value is the time before a forceable stop is used to stop the container.  If the container cannot be found by
# name or ID, a [ContainerNotFound](#ContainerNotFound)  error will be returned; otherwise, the ID of the
# container will be returned.
method RestartContainer(name: string, timeout: int) -> (container: string)

# KillContainer takes the name or ID of a container as well as a signal to be applied to the container.  Once the
# container has been killed, the container's ID is returned.  If the container cannot be found, a
# [ContainerNotFound](#ContainerNotFound) error is returned. See also [StopContainer](StopContainer).
method KillContainer(name: string, signal: int) -> (container: string)

# This method has not be implemented yet.
method UpdateContainer() -> (notimplemented: NotImplemented)

# This method has not be implemented yet.
method RenameContainer() -> (notimplemented: NotImplemented)

# PauseContainer takes the name or ID of container and pauses it.  If the container cannot be found,
# a [ContainerNotFound](#ContainerNotFound) error will be returned; otherwise the ID of the container is returned.
# See also [UnpauseContainer](UnpauseContainer).
method PauseContainer(name: string) -> (container: string)

# UnpauseContainer takes the name or ID of container and unpauses a paused container.  If the container cannot be
# found, a [ContainerNotFound](#ContainerNotFound) error will be returned; otherwise the ID of the container is returned.
# See also [PauseContainer](PauseContainer).
method UnpauseContainer(name: string) -> (container: string)

# This method has not be implemented yet.
method AttachToContainer() -> (notimplemented: NotImplemented)

# WaitContainer takes the name of ID of a container and waits until the container stops.  Upon stopping, the return
# code of the container is returned. If the container container cannot be found by ID or name,
# a [ContainerNotFound](#ContainerNotFound) error is returned.
method WaitContainer(name: string) -> (exitcode: int)

# RemoveContainer takes requires the name or ID of container as well a boolean representing whether a running
# container can be stopped and removed.  Upon sucessful removal of the container, its ID is returned.  If the
# container cannot be found by name or ID, an [ContainerNotFound](#ContainerNotFound) error will be returned.
# #### Error
# ~~~
# $ varlink call -m unix:/run/podman/io.projectatomic.podman/io.projectatomic.podman.RemoveContainer '{"name": "62f4fd98cb57"}'
# {
#   "container": "62f4fd98cb57f529831e8f90610e54bba74bd6f02920ffb485e15376ed365c20"
# }
# ~~~
method RemoveContainer(name: string, force: bool) -> (container: string)

# DeleteStoppedContainers will delete all containers that are not running. It will return a list the deleted
# container IDs.  See also [RemoveContainer](RemoveContainer).
method DeleteStoppedContainers() -> (containers: []string)

# ListImages returns an array of ImageInList structures which provide basic information about
# an image currenly in storage.  See also [InspectImage](InspectImage).
method ListImages() -> (images: []ImageInList)

# This function is not implemented yet.
method BuildImage() -> (notimplemented: NotImplemented)

# This function is not implemented yet.
method CreateImage() -> (notimplemented: NotImplemented)

# InspectImage takes the name or ID of an image and returns a string respresentation of data associated with the
#image.  You must serialize the string into JSON to use it further.  An [ImageNotFound](#ImageNotFound) error will
# be returned if the image cannot be found.
method InspectImage(name: string) -> (image: string)

# HistoryImage takes the name or ID of an image and returns information about its history and layers.  The returned
# history is in the form of an array of ImageHistory structures.  If the image cannot be found, an
# [ImageNotFound](#ImageNotFound) error is returned.
method HistoryImage(name: string) -> (history: []ImageHistory)

# PushImage takes three input arguments: the name or ID of an image, the fully-qualified destination name of the image,
# and a boolean as to whether tls-verify should be used.  It will return an [ImageNotFound](#ImageNotFound) error if
# the image cannot be found in local storage; otherwise the ID of the image will be returned on success.
method PushImage(name: string, tag: string, tlsverify: bool) -> (image: string)

# TagImage takes the name or ID of an image in local storage as well as the desired tag name.  If the image cannot
# be found, an [ImageNotFound](#ImageNotFound) error will be returned; otherwise, the ID of the image is returned on success.
method TagImage(name: string, tagged: string) -> (image: string)

# RemoveImage takes the name or ID of an image as well as a booleon that determines if containers using that image
# should be deleted.  If the image cannot be found, an [ImageNotFound](#ImageNotFound) error will be returned.  The
# ID of the removed image is returned when complete.  See also [DeleteUnusedImages](DeleteUnusedImages).
# #### Example
# ~~~
# varlink call -m unix:/run/podman/io.projectatomic.podman/io.projectatomic.podman.RemoveImage '{"name": "registry.fedoraproject.org/fedora", "force": true}'
# {
#   "image": "426866d6fa419873f97e5cbd320eeb22778244c1dfffa01c944db3114f55772e"
# }
# ~~~
method RemoveImage(name: string, force: bool) -> (image: string)

# SearchImage takes the string of an image name and a limit of searches from each registries to be returned.  SearchImage
# will then use a glob-like match to find the image you are searching for.  The images are returned in an array of
# ImageSearch structures which contain information about the image as well as its fully-qualified name.
method SearchImage(name: string, limit: int) -> (images: []ImageSearch)

# DeleteUnusedImages deletes any images not associated with a container.  The IDs of the deleted images are returned
# in a string array.
method DeleteUnusedImages() -> (images: []string)

# Commit, creates an image from an existing container. It requires the name or
# ID of the container as well as the resulting image name.  Optionally, you can define an author and message
# to be added to the resulting image.  You can also define changes to the resulting image for the following
# attributes: _CMD, ENTRYPOINT, ENV, EXPOSE, LABEL, STOPSIGNAL, USER, VOLUME, and WORKDIR_.  To pause the
# container while it is being committed, pass a _true_ bool for the pause argument.  If the container cannot
# be found by the ID or name provided, a (ContainerNotFound)[#ContainerNotFound] error will be returned; otherwise,
# the resulting image's ID will be returned as a string.
method Commit(name: string, image_name: string, changes: []string, author: string, message: string, pause: bool) -> (image: string)

# ImportImage imports an image from a source (like tarball) into local storage.  The image can have additional
# descriptions added to it using the message and changes options. See also [ExportImage](ExportImage).
method ImportImage(source: string, reference: string, message: string, changes: []string) -> (image: string)

# ExportImage takes the name or ID of an image and exports it to a destination like a tarball.  There is also
# a booleon option to force compression.  Upon completion, the ID of the image is returned. If the image cannot
# be found in local storage, an [ImageNotFound](#ImageNotFound) error will be returned. See also [ImportImage](ImportImage).
method ExportImage(name: string, destination: string, compress: bool) -> (image: string)

# PullImage pulls an image from a repository to local storage.  After the pull is successful, the ID of the image
# is returned.
# #### Example
# ~~~
# $ varlink call -m unix:/run/podman/io.projectatomic.podman/io.projectatomic.podman.PullImage '{"name": "registry.fedoraproject.org/fedora"}'
# {
#   "id": "426866d6fa419873f97e5cbd320eeb22778244c1dfffa01c944db3114f55772e"
# }
# ~~~
method PullImage(name: string) -> (id: string)


# ImageNotFound means the image could not be found by the provided name or ID in local storage.
error ImageNotFound (name: string)

# ContainerNotFound means the container could not be found by the provided name or ID in local storage.
error ContainerNotFound (name: string)

# ErrorOccurred is a generic error for an error that occurs during the execution.  The actual error message
# is includes as part of the error's text.
error ErrorOccurred (reason: string)

# RuntimeErrors generally means a runtime could not be found or gotten.
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
