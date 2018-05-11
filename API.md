# io.projectatomic.podman
Podman Service Interface and API description.  The master version of this document can be found
in the [API.md](https://github.com/projectatomic/libpod/blob/master/API.md) file in the upstream libpod repository.
## Index

[func AttachToContainer() NotImplemented](#AttachToContainer)

[func BuildImage() NotImplemented](#BuildImage)

[func Commit(name: string, image_name: string, changes: []string, author: string, message: string, pause: bool) string](#Commit)

[func CreateContainer() NotImplemented](#CreateContainer)

[func CreateImage() NotImplemented](#CreateImage)

[func DeleteStoppedContainers() []string](#DeleteStoppedContainers)

[func DeleteUnusedImages() []string](#DeleteUnusedImages)

[func ExportContainer(name: string, path: string) string](#ExportContainer)

[func ExportImage(name: string, destination: string, compress: bool) string](#ExportImage)

[func GetContainer(name: string) ListContainerData](#GetContainer)

[func GetContainerLogs(name: string) []string](#GetContainerLogs)

[func GetContainerStats(name: string) ContainerStats](#GetContainerStats)

[func GetInfo() PodmanInfo](#GetInfo)

[func GetVersion() Version](#GetVersion)

[func HistoryImage(name: string) ImageHistory](#HistoryImage)

[func ImportImage(source: string, reference: string, message: string, changes: []string) string](#ImportImage)

[func InspectContainer(name: string) string](#InspectContainer)

[func InspectImage(name: string) string](#InspectImage)

[func KillContainer(name: string, signal: int) string](#KillContainer)

[func ListContainerChanges(name: string) ContainerChanges](#ListContainerChanges)

[func ListContainerProcesses(name: string, opts: []string) []string](#ListContainerProcesses)

[func ListContainers() ListContainerData](#ListContainers)

[func ListImages() ImageInList](#ListImages)

[func PauseContainer(name: string) string](#PauseContainer)

[func Ping() StringResponse](#Ping)

[func PullImage(name: string) string](#PullImage)

[func PushImage(name: string, tag: string, tlsverify: bool) string](#PushImage)

[func RemoveContainer(name: string, force: bool) string](#RemoveContainer)

[func RemoveImage(name: string, force: bool) string](#RemoveImage)

[func RenameContainer() NotImplemented](#RenameContainer)

[func ResizeContainerTty() NotImplemented](#ResizeContainerTty)

[func RestartContainer(name: string, timeout: int) string](#RestartContainer)

[func SearchImage(name: string, limit: int) ImageSearch](#SearchImage)

[func StartContainer() NotImplemented](#StartContainer)

[func StopContainer(name: string, timeout: int) string](#StopContainer)

[func TagImage(name: string, tagged: string) string](#TagImage)

[func UnpauseContainer(name: string) string](#UnpauseContainer)

[func UpdateContainer() NotImplemented](#UpdateContainer)

[func WaitContainer(name: string) int](#WaitContainer)

[type ContainerChanges](#ContainerChanges)

[type ContainerMount](#ContainerMount)

[type ContainerNameSpace](#ContainerNameSpace)

[type ContainerPortMappings](#ContainerPortMappings)

[type ContainerStats](#ContainerStats)

[type ImageHistory](#ImageHistory)

[type ImageInList](#ImageInList)

[type ImageSearch](#ImageSearch)

[type InfoGraphStatus](#InfoGraphStatus)

[type InfoHost](#InfoHost)

[type InfoPodmanBinary](#InfoPodmanBinary)

[type InfoStore](#InfoStore)

[type ListContainerData](#ListContainerData)

[type NotImplemented](#NotImplemented)

[type PodmanInfo](#PodmanInfo)

[type StringResponse](#StringResponse)

[type Version](#Version)

[error ContainerNotFound](#ContainerNotFound)

[error ErrorOccurred](#ErrorOccurred)

[error ImageNotFound](#ImageNotFound)

[error RuntimeError](#RuntimeError)

## Methods
### <a name="AttachToContainer"></a>func AttachToContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method AttachToContainer() [NotImplemented](#NotImplemented)</div>
This method has not be implemented yet.
### <a name="BuildImage"></a>func BuildImage
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method BuildImage() [NotImplemented](#NotImplemented)</div>
This function is not implemented yet.
### <a name="Commit"></a>func Commit
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method Commit(name: [string](https://godoc.org/builtin#string), image_name: [string](https://godoc.org/builtin#string), changes: [[]string](#[]string), author: [string](https://godoc.org/builtin#string), message: [string](https://godoc.org/builtin#string), pause: [bool](https://godoc.org/builtin#bool)) [string](https://godoc.org/builtin#string)</div>
Commit, creates an image from an existing container. It requires the name or
ID of the container as well as the resulting image name.  Optionally, you can define an author and message
to be added to the resulting image.  You can also define changes to the resulting image for the following
attributes: _CMD, ENTRYPOINT, ENV, EXPOSE, LABEL, STOPSIGNAL, USER, VOLUME, and WORKDIR_.  To pause the
container while it is being committed, pass a _true_ bool for the pause argument.  If the container cannot
be found by the ID or name provided, a (ContainerNotFound)[#ContainerNotFound] error will be returned; otherwise,
the resulting image's ID will be returned as a string.
### <a name="CreateContainer"></a>func CreateContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method CreateContainer() [NotImplemented](#NotImplemented)</div>
This method is not implemented yet.
### <a name="CreateImage"></a>func CreateImage
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method CreateImage() [NotImplemented](#NotImplemented)</div>
This function is not implemented yet.
### <a name="DeleteStoppedContainers"></a>func DeleteStoppedContainers
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method DeleteStoppedContainers() [[]string](#[]string)</div>
DeleteStoppedContainers will delete all containers that are not running. It will return a list the deleted
container IDs.  See also [RemoveContainer](RemoveContainer).
### <a name="DeleteUnusedImages"></a>func DeleteUnusedImages
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method DeleteUnusedImages() [[]string](#[]string)</div>
DeleteUnusedImages deletes any images not associated with a container.  The IDs of the deleted images are returned
in a string array.
### <a name="ExportContainer"></a>func ExportContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ExportContainer(name: [string](https://godoc.org/builtin#string), path: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
ExportContainer creates an image from a container.  It takes the name or ID of a container and a
path representing the target tarfile.  If the container cannot be found, a [ContainerNotFound](#ContainerNotFound)
error will be returned.
The return value is the written tarfile.
### <a name="ExportImage"></a>func ExportImage
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ExportImage(name: [string](https://godoc.org/builtin#string), destination: [string](https://godoc.org/builtin#string), compress: [bool](https://godoc.org/builtin#bool)) [string](https://godoc.org/builtin#string)</div>
ExportImage takes the name or ID of an image and exports it to a destination like a tarball.  There is also
a booleon option to force compression.  Upon completion, the ID of the image is returned. If the image cannot
be found in local storage, an [ImageNotFound](#ImageNotFound) error will be returned. See also [ImportImage](ImportImage).
### <a name="GetContainer"></a>func GetContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method GetContainer(name: [string](https://godoc.org/builtin#string)) [ListContainerData](#ListContainerData)</div>
GetContainer takes a name or ID of a container and returns single ListContainerData
structure.  A [ContainerNotFound](#ContainerNotFound) error will be returned if the container cannot be found.
See also [ListContainers](ListContainers) and [InspectContainer](InspectContainer).
### <a name="GetContainerLogs"></a>func GetContainerLogs
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method GetContainerLogs(name: [string](https://godoc.org/builtin#string)) [[]string](#[]string)</div>
GetContainerLogs takes a name or ID of a container and returns the logs of that container.
If the container cannot be found, a [ContainerNotFound](#ContainerNotFound) error will be returned.
The container logs are returned as an array of strings.  GetContainerLogs will honor the streaming
capability of varlink if the client invokes it.
### <a name="GetContainerStats"></a>func GetContainerStats
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method GetContainerStats(name: [string](https://godoc.org/builtin#string)) [ContainerStats](#ContainerStats)</div>
GetContainerStats takes the name or ID of a container and returns a single ContainerStats structure which
contains attributes like memory and cpu usage.  If the container cannot be found, a
[ContainerNotFound](#ContainerNotFound)  error will be returned.
#### Example
~~~
$ varlink call -m unix:/run/io.projectatomic.podman/io.projectatomic.podman.GetContainerStats '{"name": "c33e4164f384"}'
{
  "container": {
    "block_input": 0,
    "block_output": 0,
    "cpu": 2.571123918839990154678e-08,
    "cpu_nano": 49037378,
    "id": "c33e4164f384aa9d979072a63319d66b74fd7a128be71fa68ede24f33ec6cfee",
    "mem_limit": 33080606720,
    "mem_perc": 2.166828456524753747370e-03,
    "mem_usage": 716800,
    "name": "competent_wozniak",
    "net_input": 768,
    "net_output": 5910,
    "pids": 1,
    "system_nano": 10000000
  }
}
~~~
### <a name="GetInfo"></a>func GetInfo
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method GetInfo() [PodmanInfo](#PodmanInfo)</div>
GetInfo returns a [PodmanInfo](#PodmanInfo) struct that describes podman and its host such as storage stats,
build information of Podman, and system-wide registries.
### <a name="GetVersion"></a>func GetVersion
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method GetVersion() [Version](#Version)</div>
GetVersion returns a Version structure describing the libpod setup on their
system.
### <a name="HistoryImage"></a>func HistoryImage
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method HistoryImage(name: [string](https://godoc.org/builtin#string)) [ImageHistory](#ImageHistory)</div>
HistoryImage takes the name or ID of an image and returns information about its history and layers.  The returned
history is in the form of an array of ImageHistory structures.  If the image cannot be found, an
[ImageNotFound](#ImageNotFound) error is returned.
### <a name="ImportImage"></a>func ImportImage
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ImportImage(source: [string](https://godoc.org/builtin#string), reference: [string](https://godoc.org/builtin#string), message: [string](https://godoc.org/builtin#string), changes: [[]string](#[]string)) [string](https://godoc.org/builtin#string)</div>
ImportImage imports an image from a source (like tarball) into local storage.  The image can have additional
descriptions added to it using the message and changes options. See also [ExportImage](ExportImage).
### <a name="InspectContainer"></a>func InspectContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method InspectContainer(name: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
InspectContainer data takes a name or ID of a container returns the inspection
data in string format.  You can then serialize the string into JSON.  A [ContainerNotFound](#ContainerNotFound)
error will be returned if the container cannot be found. See also [InspectImage](#InspectImage).
### <a name="InspectImage"></a>func InspectImage
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method InspectImage(name: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
InspectImage takes the name or ID of an image and returns a string respresentation of data associated with the
mage.  You must serialize the string into JSON to use it further.  An [ImageNotFound](#ImageNotFound) error will
be returned if the image cannot be found.
### <a name="KillContainer"></a>func KillContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method KillContainer(name: [string](https://godoc.org/builtin#string), signal: [int](https://godoc.org/builtin#int)) [string](https://godoc.org/builtin#string)</div>
KillContainer takes the name or ID of a container as well as a signal to be applied to the container.  Once the
container has been killed, the container's ID is returned.  If the container cannot be found, a
[ContainerNotFound](#ContainerNotFound) error is returned. See also [StopContainer](StopContainer).
### <a name="ListContainerChanges"></a>func ListContainerChanges
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ListContainerChanges(name: [string](https://godoc.org/builtin#string)) [ContainerChanges](#ContainerChanges)</div>
ListContainerChanges takes a name or ID of a container and returns changes between the container and
its base image. It returns a struct of changed, deleted, and added path names. If the
container cannot be found, a [ContainerNotFound](#ContainerNotFound) error will be returned.
### <a name="ListContainerProcesses"></a>func ListContainerProcesses
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ListContainerProcesses(name: [string](https://godoc.org/builtin#string), opts: [[]string](#[]string)) [[]string](#[]string)</div>
ListContainerProcesses takes a name or ID of a container and returns the processes
running inside the container as array of strings.  It will accept an array of string
arguements that represent ps options.  If the container cannot be found, a [ContainerNotFound](#ContainerNotFound)
error will be returned.
#### Example
~~~
$ varlink call -m unix:/run/io.projectatomic.podman/io.projectatomic.podman.ListContainerProcesses '{"name": "135d71b9495f", "opts": []}'
{
  "container": [
    "  UID   PID  PPID  C STIME TTY          TIME CMD",
    "    0 21220 21210  0 09:05 pts/0    00:00:00 /bin/sh",
    "    0 21232 21220  0 09:05 pts/0    00:00:00 top",
    "    0 21284 21220  0 09:05 pts/0    00:00:00 vi /etc/hosts"
  ]
}
~~~
### <a name="ListContainers"></a>func ListContainers
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ListContainers() [ListContainerData](#ListContainerData)</div>
ListContainers returns a list of containers in no particular order.  There are
returned as an array of ListContainerData structs.  See also [GetContainer](#GetContainer).
### <a name="ListImages"></a>func ListImages
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ListImages() [ImageInList](#ImageInList)</div>
ListImages returns an array of ImageInList structures which provide basic information about
an image currenly in storage.  See also [InspectImage](InspectImage).
### <a name="PauseContainer"></a>func PauseContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method PauseContainer(name: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
PauseContainer takes the name or ID of container and pauses it.  If the container cannot be found,
a [ContainerNotFound](#ContainerNotFound) error will be returned; otherwise the ID of the container is returned.
See also [UnpauseContainer](UnpauseContainer).
### <a name="Ping"></a>func Ping
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method Ping() [StringResponse](#StringResponse)</div>
Ping provides a response for developers to ensure their varlink setup is working.
#### Example
~~~
$ varlink call -m unix:/run/io.projectatomic.podman/io.projectatomic.podman.Ping
{
  "ping": {
    "message": "OK"
  }
}
~~~
### <a name="PullImage"></a>func PullImage
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method PullImage(name: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
PullImage pulls an image from a repository to local storage.  After the pull is successful, the ID of the image
is returned.
#### Example
~~~
$ varlink call -m unix:/run/io.projectatomic.podman/io.projectatomic.podman.PullImage '{"name": "registry.fedoraproject.org/fedora"}'
{
  "id": "426866d6fa419873f97e5cbd320eeb22778244c1dfffa01c944db3114f55772e"
}
~~~
### <a name="PushImage"></a>func PushImage
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method PushImage(name: [string](https://godoc.org/builtin#string), tag: [string](https://godoc.org/builtin#string), tlsverify: [bool](https://godoc.org/builtin#bool)) [string](https://godoc.org/builtin#string)</div>
PushImage takes three input arguments: the name or ID of an image, the fully-qualified destination name of the image,
and a boolean as to whether tls-verify should be used.  It will return an [ImageNotFound](#ImageNotFound) error if
the image cannot be found in local storage; otherwise the ID of the image will be returned on success.
### <a name="RemoveContainer"></a>func RemoveContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method RemoveContainer(name: [string](https://godoc.org/builtin#string), force: [bool](https://godoc.org/builtin#bool)) [string](https://godoc.org/builtin#string)</div>
RemoveContainer takes requires the name or ID of container as well a boolean representing whether a running
container can be stopped and removed.  Upon sucessful removal of the container, its ID is returned.  If the
container cannot be found by name or ID, an [ContainerNotFound](#ContainerNotFound) error will be returned.
#### Error
~~~
$ varlink call -m unix:/run/io.projectatomic.podman/io.projectatomic.podman.RemoveContainer '{"name": "62f4fd98cb57"}'
{
  "container": "62f4fd98cb57f529831e8f90610e54bba74bd6f02920ffb485e15376ed365c20"
}
~~~
### <a name="RemoveImage"></a>func RemoveImage
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method RemoveImage(name: [string](https://godoc.org/builtin#string), force: [bool](https://godoc.org/builtin#bool)) [string](https://godoc.org/builtin#string)</div>
RemoveImage takes the name or ID of an image as well as a booleon that determines if containers using that image
should be deleted.  If the image cannot be found, an [ImageNotFound](#ImageNotFound) error will be returned.  The
ID of the removed image is returned when complete.  See also [DeleteUnusedImages](DeleteUnusedImages).
#### Example
~~~
varlink call -m unix:/run/io.projectatomic.podman/io.projectatomic.podman.RemoveImage '{"name": "registry.fedoraproject.org/fedora", "force": true}'
{
  "image": "426866d6fa419873f97e5cbd320eeb22778244c1dfffa01c944db3114f55772e"
}
~~~
### <a name="RenameContainer"></a>func RenameContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method RenameContainer() [NotImplemented](#NotImplemented)</div>
This method has not be implemented yet.
### <a name="ResizeContainerTty"></a>func ResizeContainerTty
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ResizeContainerTty() [NotImplemented](#NotImplemented)</div>
This method has not be implemented yet.
### <a name="RestartContainer"></a>func RestartContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method RestartContainer(name: [string](https://godoc.org/builtin#string), timeout: [int](https://godoc.org/builtin#int)) [string](https://godoc.org/builtin#string)</div>
RestartContainer will restart a running container given a container name or ID and timeout value. The timeout
value is the time before a forceable stop is used to stop the container.  If the container cannot be found by
name or ID, a [ContainerNotFound](#ContainerNotFound)  error will be returned; otherwise, the ID of the
container will be returned.
### <a name="SearchImage"></a>func SearchImage
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method SearchImage(name: [string](https://godoc.org/builtin#string), limit: [int](https://godoc.org/builtin#int)) [ImageSearch](#ImageSearch)</div>
SearchImage takes the string of an image name and a limit of searches from each registries to be returned.  SearchImage
will then use a glob-like match to find the image you are searching for.  The images are returned in an array of
ImageSearch structures which contain information about the image as well as its fully-qualified name.
### <a name="StartContainer"></a>func StartContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method StartContainer() [NotImplemented](#NotImplemented)</div>
This method has not be implemented yet.
### <a name="StopContainer"></a>func StopContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method StopContainer(name: [string](https://godoc.org/builtin#string), timeout: [int](https://godoc.org/builtin#int)) [string](https://godoc.org/builtin#string)</div>
StopContainer stops a container given a timeout.  It takes the name or ID of a container as well as a
timeout value.  The timeout value the time before a forceable stop to the container is applied.  It
returns the container ID once stopped. If the container cannot be found, a [ContainerNotFound](#ContainerNotFound)
error will be returned instead. See also [KillContainer](KillContainer).
#### Error
~~~
$ varlink call -m unix:/run/io.projectatomic.podman/io.projectatomic.podman.StopContainer '{"name": "135d71b9495f", "timeout": 5}'
{
  "container": "135d71b9495f7c3967f536edad57750bfdb569336cd107d8aabab45565ffcfb6"
}
~~~
### <a name="TagImage"></a>func TagImage
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method TagImage(name: [string](https://godoc.org/builtin#string), tagged: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
TagImage takes the name or ID of an image in local storage as well as the desired tag name.  If the image cannot
be found, an [ImageNotFound](#ImageNotFound) error will be returned; otherwise, the ID of the image is returned on success.
### <a name="UnpauseContainer"></a>func UnpauseContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method UnpauseContainer(name: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
UnpauseContainer takes the name or ID of container and unpauses a paused container.  If the container cannot be
found, a [ContainerNotFound](#ContainerNotFound) error will be returned; otherwise the ID of the container is returned.
See also [PauseContainer](PauseContainer).
### <a name="UpdateContainer"></a>func UpdateContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method UpdateContainer() [NotImplemented](#NotImplemented)</div>
This method has not be implemented yet.
### <a name="WaitContainer"></a>func WaitContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method WaitContainer(name: [string](https://godoc.org/builtin#string)) [int](https://godoc.org/builtin#int)</div>
WaitContainer takes the name of ID of a container and waits until the container stops.  Upon stopping, the return
code of the container is returned. If the container container cannot be found by ID or name,
a [ContainerNotFound](#ContainerNotFound) error is returned.
## Types
### <a name="ContainerChanges"></a>type ContainerChanges

ContainerChanges describes the return struct for ListContainerChanges

changed [[]string](#[]string)

added [[]string](#[]string)

deleted [[]string](#[]string)
### <a name="ContainerMount"></a>type ContainerMount

ContainerMount describes the struct for mounts in a container

destination [string](https://godoc.org/builtin#string)

type [string](https://godoc.org/builtin#string)

source [string](https://godoc.org/builtin#string)

options [[]string](#[]string)
### <a name="ContainerNameSpace"></a>type ContainerNameSpace

ContainerNamespace describes the namespace structure for an existing container

user [string](https://godoc.org/builtin#string)

uts [string](https://godoc.org/builtin#string)

pidns [string](https://godoc.org/builtin#string)

pid [string](https://godoc.org/builtin#string)

cgroup [string](https://godoc.org/builtin#string)

net [string](https://godoc.org/builtin#string)

mnt [string](https://godoc.org/builtin#string)

ipc [string](https://godoc.org/builtin#string)
### <a name="ContainerPortMappings"></a>type ContainerPortMappings

ContainerPortMappings describes the struct for portmappings in an existing container

host_port [string](https://godoc.org/builtin#string)

host_ip [string](https://godoc.org/builtin#string)

protocol [string](https://godoc.org/builtin#string)

container_port [string](https://godoc.org/builtin#string)
### <a name="ContainerStats"></a>type ContainerStats

ContainerStats is the return struct for the stats of a container

id [string](https://godoc.org/builtin#string)

name [string](https://godoc.org/builtin#string)

cpu [float](https://golang.org/src/builtin/builtin.go#L58)

cpu_nano [int](https://godoc.org/builtin#int)

system_nano [int](https://godoc.org/builtin#int)

mem_usage [int](https://godoc.org/builtin#int)

mem_limit [int](https://godoc.org/builtin#int)

mem_perc [float](https://golang.org/src/builtin/builtin.go#L58)

net_input [int](https://godoc.org/builtin#int)

net_output [int](https://godoc.org/builtin#int)

block_output [int](https://godoc.org/builtin#int)

block_input [int](https://godoc.org/builtin#int)

pids [int](https://godoc.org/builtin#int)
### <a name="ImageHistory"></a>type ImageHistory

ImageHistory describes the returned structure from ImageHistory.

id [string](https://godoc.org/builtin#string)

created [string](https://godoc.org/builtin#string)

createdBy [string](https://godoc.org/builtin#string)

tags [[]string](#[]string)

size [int](https://godoc.org/builtin#int)

comment [string](https://godoc.org/builtin#string)
### <a name="ImageInList"></a>type ImageInList

ImageInList describes the structure that is returned in
ListImages.

id [string](https://godoc.org/builtin#string)

parentId [string](https://godoc.org/builtin#string)

repoTags [[]string](#[]string)

repoDigests [[]string](#[]string)

created [string](https://godoc.org/builtin#string)

size [int](https://godoc.org/builtin#int)

virtualSize [int](https://godoc.org/builtin#int)

containers [int](https://godoc.org/builtin#int)

labels [map[string]](#map[string])
### <a name="ImageSearch"></a>type ImageSearch

ImageSearch is the returned structure for SearchImage.  It is returned
in arrary form.

description [string](https://godoc.org/builtin#string)

is_official [bool](https://godoc.org/builtin#bool)

is_automated [bool](https://godoc.org/builtin#bool)

name [string](https://godoc.org/builtin#string)

star_count [int](https://godoc.org/builtin#int)
### <a name="InfoGraphStatus"></a>type InfoGraphStatus

InfoGraphStatus describes the detailed status of the storage driver

backing_filesystem [string](https://godoc.org/builtin#string)

native_overlay_diff [string](https://godoc.org/builtin#string)

supports_d_type [string](https://godoc.org/builtin#string)
### <a name="InfoHost"></a>type InfoHost

InfoHost describes the host stats portion of PodmanInfo

mem_free [int](https://godoc.org/builtin#int)

mem_total [int](https://godoc.org/builtin#int)

swap_free [int](https://godoc.org/builtin#int)

swap_total [int](https://godoc.org/builtin#int)

arch [string](https://godoc.org/builtin#string)

cpus [int](https://godoc.org/builtin#int)

hostname [string](https://godoc.org/builtin#string)

kernel [string](https://godoc.org/builtin#string)

os [string](https://godoc.org/builtin#string)

uptime [string](https://godoc.org/builtin#string)
### <a name="InfoPodmanBinary"></a>type InfoPodmanBinary

InfoPodman provides details on the podman binary

compiler [string](https://godoc.org/builtin#string)

go_version [string](https://godoc.org/builtin#string)

podman_version [string](https://godoc.org/builtin#string)

git_commit [string](https://godoc.org/builtin#string)
### <a name="InfoStore"></a>type InfoStore

InfoStore describes the host's storage informatoin

containers [int](https://godoc.org/builtin#int)

images [int](https://godoc.org/builtin#int)

graph_driver_name [string](https://godoc.org/builtin#string)

graph_driver_options [string](https://godoc.org/builtin#string)

graph_root [string](https://godoc.org/builtin#string)

graph_status [InfoGraphStatus](#InfoGraphStatus)

run_root [string](https://godoc.org/builtin#string)
### <a name="ListContainerData"></a>type ListContainerData

ListContainer is the returned struct for an individual container

id [string](https://godoc.org/builtin#string)

image [string](https://godoc.org/builtin#string)

imageid [string](https://godoc.org/builtin#string)

command [[]string](#[]string)

createdat [string](https://godoc.org/builtin#string)

runningfor [string](https://godoc.org/builtin#string)

status [string](https://godoc.org/builtin#string)

ports [ContainerPortMappings](#ContainerPortMappings)

rootfssize [int](https://godoc.org/builtin#int)

rwsize [int](https://godoc.org/builtin#int)

names [string](https://godoc.org/builtin#string)

labels [map[string]](#map[string])

mounts [ContainerMount](#ContainerMount)

containerrunning [bool](https://godoc.org/builtin#bool)

namespaces [ContainerNameSpace](#ContainerNameSpace)
### <a name="NotImplemented"></a>type NotImplemented



comment [string](https://godoc.org/builtin#string)
### <a name="PodmanInfo"></a>type PodmanInfo

PodmanInfo describes the Podman host and build

host [InfoHost](#InfoHost)

registries [[]string](#[]string)

insecure_registries [[]string](#[]string)

store [InfoStore](#InfoStore)

podman [InfoPodmanBinary](#InfoPodmanBinary)
### <a name="StringResponse"></a>type StringResponse



message [string](https://godoc.org/builtin#string)
### <a name="Version"></a>type Version

Version is the structure returned by GetVersion

version [string](https://godoc.org/builtin#string)

go_version [string](https://godoc.org/builtin#string)

git_commit [string](https://godoc.org/builtin#string)

built [int](https://godoc.org/builtin#int)

os_arch [string](https://godoc.org/builtin#string)
## Errors
### <a name="ContainerNotFound"></a>type ContainerNotFound

ContainerNotFound means the container could not be found by the provided name or ID in local storage.
### <a name="ErrorOccurred"></a>type ErrorOccurred

ErrorOccurred is a generic error for an error that occurs during the execution.  The actual error message
is includes as part of the error's text.
### <a name="ImageNotFound"></a>type ImageNotFound

ImageNotFound means the image could not be found by the provided name or ID in local storage.
### <a name="RuntimeError"></a>type RuntimeError

RuntimeErrors generally means a runtime could not be found or gotten.
