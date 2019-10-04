# io.podman
Podman Service Interface and API description.  The master version of this document can be found
in the [API.md](https://github.com/containers/libpod/blob/master/API.md) file in the upstream libpod repository.
## Index

[func Attach(name: string, detachKeys: string, start: bool) ](#Attach)

[func AttachControl(name: string) ](#AttachControl)

[func BuildImage(build: BuildInfo) MoreResponse](#BuildImage)

[func BuildImageHierarchyMap(name: string) string](#BuildImageHierarchyMap)

[func Commit(name: string, image_name: string, changes: []string, author: string, message: string, pause: bool, manifestType: string) MoreResponse](#Commit)

[func ContainerArtifacts(name: string, artifactName: string) string](#ContainerArtifacts)

[func ContainerCheckpoint(name: string, keep: bool, leaveRunning: bool, tcpEstablished: bool) string](#ContainerCheckpoint)

[func ContainerConfig(name: string) string](#ContainerConfig)

[func ContainerExists(name: string) int](#ContainerExists)

[func ContainerInspectData(name: string, size: bool) string](#ContainerInspectData)

[func ContainerRestore(name: string, keep: bool, tcpEstablished: bool) string](#ContainerRestore)

[func ContainerRunlabel(runlabel: Runlabel) ](#ContainerRunlabel)

[func ContainerStateData(name: string) string](#ContainerStateData)

[func CreateContainer(create: Create) string](#CreateContainer)

[func CreateFromCC(in: []string) string](#CreateFromCC)

[func CreatePod(create: PodCreate) string](#CreatePod)

[func DeleteStoppedContainers() []string](#DeleteStoppedContainers)

[func DeleteUnusedImages() []string](#DeleteUnusedImages)

[func Diff(name: string) DiffInfo](#Diff)

[func EvictContainer(name: string, removeVolumes: bool) string](#EvictContainer)

[func ExecContainer(opts: ExecOpts) ](#ExecContainer)

[func ExportContainer(name: string, path: string) string](#ExportContainer)

[func ExportImage(name: string, destination: string, compress: bool, tags: []string) string](#ExportImage)

[func GenerateKube(name: string, service: bool) KubePodService](#GenerateKube)

[func GetAttachSockets(name: string) Sockets](#GetAttachSockets)

[func GetContainer(id: string) Container](#GetContainer)

[func GetContainerLogs(name: string) []string](#GetContainerLogs)

[func GetContainerStats(name: string) ContainerStats](#GetContainerStats)

[func GetContainerStatsWithHistory(previousStats: ContainerStats) ContainerStats](#GetContainerStatsWithHistory)

[func GetContainersByContext(all: bool, latest: bool, args: []string) []string](#GetContainersByContext)

[func GetContainersByStatus(status: []string) Container](#GetContainersByStatus)

[func GetContainersLogs(names: []string, follow: bool, latest: bool, since: string, tail: int, timestamps: bool) LogLine](#GetContainersLogs)

[func GetEvents(filter: []string, since: string, until: string) Event](#GetEvents)

[func GetImage(id: string) Image](#GetImage)

[func GetInfo() PodmanInfo](#GetInfo)

[func GetLayersMapWithImageInfo() string](#GetLayersMapWithImageInfo)

[func GetPod(name: string) ListPodData](#GetPod)

[func GetPodStats(name: string) string, ContainerStats](#GetPodStats)

[func GetPodsByContext(all: bool, latest: bool, args: []string) []string](#GetPodsByContext)

[func GetPodsByStatus(statuses: []string) []string](#GetPodsByStatus)

[func GetVersion() string, string, string, string, string, int](#GetVersion)

[func GetVolumes(args: []string, all: bool) Volume](#GetVolumes)

[func HealthCheckRun(nameOrID: string) string](#HealthCheckRun)

[func HistoryImage(name: string) ImageHistory](#HistoryImage)

[func ImageExists(name: string) int](#ImageExists)

[func ImageSave(options: ImageSaveOptions) MoreResponse](#ImageSave)

[func ImagesPrune(all: bool) []string](#ImagesPrune)

[func ImportImage(source: string, reference: string, message: string, changes: []string, delete: bool) string](#ImportImage)

[func InitContainer(name: string) string](#InitContainer)

[func InspectContainer(name: string) string](#InspectContainer)

[func InspectImage(name: string) string](#InspectImage)

[func InspectPod(name: string) string](#InspectPod)

[func KillContainer(name: string, signal: int) string](#KillContainer)

[func KillPod(name: string, signal: int) string](#KillPod)

[func ListContainerChanges(name: string) ContainerChanges](#ListContainerChanges)

[func ListContainerMounts() map[string]](#ListContainerMounts)

[func ListContainerProcesses(name: string, opts: []string) []string](#ListContainerProcesses)

[func ListContainers() Container](#ListContainers)

[func ListImages() Image](#ListImages)

[func ListPods() ListPodData](#ListPods)

[func LoadImage(name: string, inputFile: string, quiet: bool, deleteFile: bool) MoreResponse](#LoadImage)

[func MountContainer(name: string) string](#MountContainer)

[func PauseContainer(name: string) string](#PauseContainer)

[func PausePod(name: string) string](#PausePod)

[func PodStateData(name: string) string](#PodStateData)

[func Ps(opts: PsOpts) PsContainer](#Ps)

[func PullImage(name: string) MoreResponse](#PullImage)

[func PushImage(name: string, tag: string, compress: bool, format: string, removeSignatures: bool, signBy: string) MoreResponse](#PushImage)

[func ReceiveFile(path: string, delete: bool) int](#ReceiveFile)

[func RemoveContainer(name: string, force: bool, removeVolumes: bool) string](#RemoveContainer)

[func RemoveImage(name: string, force: bool) string](#RemoveImage)

[func RemovePod(name: string, force: bool) string](#RemovePod)

[func RestartContainer(name: string, timeout: int) string](#RestartContainer)

[func RestartPod(name: string) string](#RestartPod)

[func SearchImages(query: string, limit: ?int, filter: ImageSearchFilter) ImageSearchResult](#SearchImages)

[func SendFile(type: string, length: int) string](#SendFile)

[func Spec(name: string) string](#Spec)

[func StartContainer(name: string) string](#StartContainer)

[func StartPod(name: string) string](#StartPod)

[func StopContainer(name: string, timeout: int) string](#StopContainer)

[func StopPod(name: string, timeout: int) string](#StopPod)

[func TagImage(name: string, tagged: string) string](#TagImage)

[func Top(nameOrID: string, descriptors: []string) []string](#Top)

[func TopPod(pod: string, latest: bool, descriptors: []string) []string](#TopPod)

[func UnmountContainer(name: string, force: bool) ](#UnmountContainer)

[func UnpauseContainer(name: string) string](#UnpauseContainer)

[func UnpausePod(name: string) string](#UnpausePod)

[func VolumeCreate(options: VolumeCreateOpts) string](#VolumeCreate)

[func VolumeRemove(options: VolumeRemoveOpts) []string, map[string]](#VolumeRemove)

[func VolumesPrune() []string, []string](#VolumesPrune)

[func WaitContainer(name: string, interval: int) int](#WaitContainer)

[type BuildInfo](#BuildInfo)

[type BuildOptions](#BuildOptions)

[type Container](#Container)

[type ContainerChanges](#ContainerChanges)

[type ContainerMount](#ContainerMount)

[type ContainerNameSpace](#ContainerNameSpace)

[type ContainerPortMappings](#ContainerPortMappings)

[type ContainerStats](#ContainerStats)

[type Create](#Create)

[type DiffInfo](#DiffInfo)

[type Event](#Event)

[type ExecOpts](#ExecOpts)

[type Image](#Image)

[type ImageHistory](#ImageHistory)

[type ImageSaveOptions](#ImageSaveOptions)

[type ImageSearchFilter](#ImageSearchFilter)

[type ImageSearchResult](#ImageSearchResult)

[type InfoDistribution](#InfoDistribution)

[type InfoGraphStatus](#InfoGraphStatus)

[type InfoHost](#InfoHost)

[type InfoPodmanBinary](#InfoPodmanBinary)

[type InfoStore](#InfoStore)

[type KubePodService](#KubePodService)

[type ListPodContainerInfo](#ListPodContainerInfo)

[type ListPodData](#ListPodData)

[type LogLine](#LogLine)

[type MoreResponse](#MoreResponse)

[type NotImplemented](#NotImplemented)

[type PodContainerErrorData](#PodContainerErrorData)

[type PodCreate](#PodCreate)

[type PodmanInfo](#PodmanInfo)

[type PsContainer](#PsContainer)

[type PsOpts](#PsOpts)

[type Runlabel](#Runlabel)

[type Sockets](#Sockets)

[type StringResponse](#StringResponse)

[type Volume](#Volume)

[type VolumeCreateOpts](#VolumeCreateOpts)

[type VolumeRemoveOpts](#VolumeRemoveOpts)

[error ContainerNotFound](#ContainerNotFound)

[error ErrCtrStopped](#ErrCtrStopped)

[error ErrRequiresCgroupsV2ForRootless](#ErrRequiresCgroupsV2ForRootless)

[error ErrorOccurred](#ErrorOccurred)

[error ImageNotFound](#ImageNotFound)

[error InvalidState](#InvalidState)

[error NoContainerRunning](#NoContainerRunning)

[error NoContainersInPod](#NoContainersInPod)

[error PodContainerError](#PodContainerError)

[error PodNotFound](#PodNotFound)

[error RuntimeError](#RuntimeError)

[error VolumeNotFound](#VolumeNotFound)

[error WantsMoreRequired](#WantsMoreRequired)

## Methods
### <a name="Attach"></a>func Attach
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method Attach(name: [string](https://godoc.org/builtin#string), detachKeys: [string](https://godoc.org/builtin#string), start: [bool](https://godoc.org/builtin#bool)) </div>
Attach takes the name or ID of a container and sets up the ability to remotely attach to its console. The start
bool is whether you wish to start the container in question first.
### <a name="AttachControl"></a>func AttachControl
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method AttachControl(name: [string](https://godoc.org/builtin#string)) </div>

### <a name="BuildImage"></a>func BuildImage
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method BuildImage(build: [BuildInfo](#BuildInfo)) [MoreResponse](#MoreResponse)</div>
BuildImage takes a [BuildInfo](#BuildInfo) structure and builds an image.  At a minimum, you must provide the
contextDir tarball path, the 'dockerfiles' path, and 'output' option in the BuildInfo structure.  The 'output'
options is the name of the of the resulting build. It will return a [MoreResponse](#MoreResponse) structure
that contains the build logs and resulting image ID.
#### Example
~~~
$ sudo varlink call -m unix:///run/podman/io.podman/io.podman.BuildImage '{"build":{"contextDir":"/tmp/t/context.tar","dockerfiles":["Dockerfile"], "output":"foobar"}}'
{
 "image": {
   "id": "",
   "logs": [
     "STEP 1: FROM alpine\n"
   ]
 }
}
{
 "image": {
   "id": "",
   "logs": [
     "STEP 2: COMMIT foobar\n"
   ]
 }
}
{
 "image": {
   "id": "",
   "logs": [
     "b7b28af77ffec6054d13378df4fdf02725830086c7444d9c278af25312aa39b9\n"
   ]
 }
}
{
 "image": {
   "id": "b7b28af77ffec6054d13378df4fdf02725830086c7444d9c278af25312aa39b9",
   "logs": []
 }
}
~~~
### <a name="BuildImageHierarchyMap"></a>func BuildImageHierarchyMap
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method BuildImageHierarchyMap(name: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
BuildImageHierarchyMap is for the development of Podman and should not be used.
### <a name="Commit"></a>func Commit
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method Commit(name: [string](https://godoc.org/builtin#string), image_name: [string](https://godoc.org/builtin#string), changes: [[]string](#[]string), author: [string](https://godoc.org/builtin#string), message: [string](https://godoc.org/builtin#string), pause: [bool](https://godoc.org/builtin#bool), manifestType: [string](https://godoc.org/builtin#string)) [MoreResponse](#MoreResponse)</div>
Commit, creates an image from an existing container. It requires the name or
ID of the container as well as the resulting image name.  Optionally, you can define an author and message
to be added to the resulting image.  You can also define changes to the resulting image for the following
attributes: _CMD, ENTRYPOINT, ENV, EXPOSE, LABEL, ONBUILD, STOPSIGNAL, USER, VOLUME, and WORKDIR_.  To pause the
container while it is being committed, pass a _true_ bool for the pause argument.  If the container cannot
be found by the ID or name provided, a (ContainerNotFound)[#ContainerNotFound] error will be returned; otherwise,
the resulting image's ID will be returned as a string inside a MoreResponse.
### <a name="ContainerArtifacts"></a>func ContainerArtifacts
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ContainerArtifacts(name: [string](https://godoc.org/builtin#string), artifactName: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
ContainerArtifacts returns a container's artifacts in string form.  This call is for
development of Podman only and generally should not be used.
### <a name="ContainerCheckpoint"></a>func ContainerCheckpoint
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ContainerCheckpoint(name: [string](https://godoc.org/builtin#string), keep: [bool](https://godoc.org/builtin#bool), leaveRunning: [bool](https://godoc.org/builtin#bool), tcpEstablished: [bool](https://godoc.org/builtin#bool)) [string](https://godoc.org/builtin#string)</div>
ContainerCheckPoint performs a checkpopint on a container by its name or full/partial container
ID.  On successful checkpoint, the id of the checkpointed container is returned.
### <a name="ContainerConfig"></a>func ContainerConfig
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ContainerConfig(name: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
ContainerConfig returns a container's config in string form. This call is for
development of Podman only and generally should not be used.
### <a name="ContainerExists"></a>func ContainerExists
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ContainerExists(name: [string](https://godoc.org/builtin#string)) [int](https://godoc.org/builtin#int)</div>
ContainerExists takes a full or partial container ID or name and returns an int as to
whether the container exists in local storage.  A result of 0 means the container does
exists; whereas a result of 1 means it could not be found.
#### Example
~~~
$ varlink call -m unix:/run/podman/io.podman/io.podman.ContainerExists '{"name": "flamboyant_payne"}'{
  "exists": 0
}
~~~
### <a name="ContainerInspectData"></a>func ContainerInspectData
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ContainerInspectData(name: [string](https://godoc.org/builtin#string), size: [bool](https://godoc.org/builtin#bool)) [string](https://godoc.org/builtin#string)</div>
ContainerInspectData returns a container's inspect data in string form.  This call is for
development of Podman only and generally should not be used.
### <a name="ContainerRestore"></a>func ContainerRestore
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ContainerRestore(name: [string](https://godoc.org/builtin#string), keep: [bool](https://godoc.org/builtin#bool), tcpEstablished: [bool](https://godoc.org/builtin#bool)) [string](https://godoc.org/builtin#string)</div>
ContainerRestore restores a container that has been checkpointed.  The container to be restored can
be identified by its name or full/partial container ID.  A successful restore will result in the return
of the container's ID.
### <a name="ContainerRunlabel"></a>func ContainerRunlabel
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ContainerRunlabel(runlabel: [Runlabel](#Runlabel)) </div>
ContainerRunlabel runs executes a command as described by a given container image label.
### <a name="ContainerStateData"></a>func ContainerStateData
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ContainerStateData(name: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
ContainerStateData returns a container's state config in string form.  This call is for
development of Podman only and generally should not be used.
### <a name="CreateContainer"></a>func CreateContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method CreateContainer(create: [Create](#Create)) [string](https://godoc.org/builtin#string)</div>
CreateContainer creates a new container from an image.  It uses a [Create](#Create) type for input.
### <a name="CreateFromCC"></a>func CreateFromCC
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method CreateFromCC(in: [[]string](#[]string)) [string](https://godoc.org/builtin#string)</div>
This call is for the development of Podman only and should not be used.
### <a name="CreatePod"></a>func CreatePod
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method CreatePod(create: [PodCreate](#PodCreate)) [string](https://godoc.org/builtin#string)</div>
CreatePod creates a new empty pod.  It uses a [PodCreate](#PodCreate) type for input.
On success, the ID of the newly created pod will be returned.
#### Example
~~~
$ varlink call unix:/run/podman/io.podman/io.podman.CreatePod '{"create": {"name": "test"}}'
{
  "pod": "b05dee7bd4ccfee688099fe1588a7a898d6ddd6897de9251d4671c9b0feacb2a"
}
# $ varlink call unix:/run/podman/io.podman/io.podman.CreatePod '{"create": {"infra": true, "share": ["ipc", "net", "uts"]}}'
{
  "pod": "d7697449a8035f613c1a8891286502aca68fff7d5d49a85279b3bda229af3b28"
}
~~~
### <a name="DeleteStoppedContainers"></a>func DeleteStoppedContainers
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method DeleteStoppedContainers() [[]string](#[]string)</div>
DeleteStoppedContainers will delete all containers that are not running. It will return a list the deleted
container IDs.  See also [RemoveContainer](RemoveContainer).
#### Example
~~~
$ varlink call -m unix:/run/podman/io.podman/io.podman.DeleteStoppedContainers
{
  "containers": [
    "451410b931d00def8aa9b4f8084e4d4a39e5e04ea61f358cf53a5cf95afcdcee",
    "8b60f754a3e01389494a9581ade97d35c2765b6e2f19acd2d3040c82a32d1bc0",
    "cf2e99d4d3cad6073df199ed32bbe64b124f3e1aba6d78821aa8460e70d30084",
    "db901a329587312366e5ecff583d08f0875b4b79294322df67d90fc6eed08fc1"
  ]
}
~~~
### <a name="DeleteUnusedImages"></a>func DeleteUnusedImages
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method DeleteUnusedImages() [[]string](#[]string)</div>
DeleteUnusedImages deletes any images not associated with a container.  The IDs of the deleted images are returned
in a string array.
#### Example
~~~
$ varlink call -m unix:/run/podman/io.podman/io.podman.DeleteUnusedImages
{
  "images": [
    "166ea6588079559c724c15223f52927f514f73dd5c5cf2ae2d143e3b2e6e9b52",
    "da86e6ba6ca197bf6bc5e9d900febd906b133eaa4750e6bed647b0fbe50ed43e",
    "3ef70f7291f47dfe2b82931a993e16f5a44a0e7a68034c3e0e086d77f5829adc",
    "59788edf1f3e78cd0ebe6ce1446e9d10788225db3dedcfd1a59f764bad2b2690"
  ]
}
~~~
### <a name="Diff"></a>func Diff
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method Diff(name: [string](https://godoc.org/builtin#string)) [DiffInfo](#DiffInfo)</div>
Diff returns a diff between libpod objects
### <a name="EvictContainer"></a>func EvictContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method EvictContainer(name: [string](https://godoc.org/builtin#string), removeVolumes: [bool](https://godoc.org/builtin#bool)) [string](https://godoc.org/builtin#string)</div>
EvictContainer requires the name or ID of a container as well as a boolean that
indicates to remove builtin volumes. Upon successful eviction of the container,
its ID is returned.  If the container cannot be found by name or ID,
a [ContainerNotFound](#ContainerNotFound) error will be returned.
See also [RemoveContainer](RemoveContainer).
#### Example
~~~
$ varlink call -m unix:/run/podman/io.podman/io.podman.EvictContainer '{"name": "62f4fd98cb57"}'
{
  "container": "62f4fd98cb57f529831e8f90610e54bba74bd6f02920ffb485e15376ed365c20"
}
~~~
### <a name="ExecContainer"></a>func ExecContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ExecContainer(opts: [ExecOpts](#ExecOpts)) </div>
ExecContainer executes a command in the given container.
### <a name="ExportContainer"></a>func ExportContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ExportContainer(name: [string](https://godoc.org/builtin#string), path: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
ExportContainer creates an image from a container.  It takes the name or ID of a container and a
path representing the target tarfile.  If the container cannot be found, a [ContainerNotFound](#ContainerNotFound)
error will be returned.
The return value is the written tarfile.
#### Example
~~~
$ varlink call -m unix:/run/podman/io.podman/io.podman.ExportContainer '{"name": "flamboyant_payne", "path": "/tmp/payne.tar" }'
{
  "tarfile": "/tmp/payne.tar"
}
~~~
### <a name="ExportImage"></a>func ExportImage
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ExportImage(name: [string](https://godoc.org/builtin#string), destination: [string](https://godoc.org/builtin#string), compress: [bool](https://godoc.org/builtin#bool), tags: [[]string](#[]string)) [string](https://godoc.org/builtin#string)</div>
ExportImage takes the name or ID of an image and exports it to a destination like a tarball.  There is also
a boolean option to force compression.  It also takes in a string array of tags to be able to save multiple
tags of the same image to a tarball (each tag should be of the form <image>:<tag>).  Upon completion, the ID
of the image is returned. If the image cannot be found in local storage, an [ImageNotFound](#ImageNotFound)
error will be returned. See also [ImportImage](ImportImage).
### <a name="GenerateKube"></a>func GenerateKube
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method GenerateKube(name: [string](https://godoc.org/builtin#string), service: [bool](https://godoc.org/builtin#bool)) [KubePodService](#KubePodService)</div>
GenerateKube generates a Kubernetes v1 Pod description of a Podman container or pod
and its containers. The description is in YAML.  See also [ReplayKube](ReplayKube).
### <a name="GetAttachSockets"></a>func GetAttachSockets
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method GetAttachSockets(name: [string](https://godoc.org/builtin#string)) [Sockets](#Sockets)</div>
GetAttachSockets takes the name or ID of an existing container.  It returns file paths for two sockets needed
to properly communicate with a container.  The first is the actual I/O socket that the container uses.  The
second is a "control" socket where things like resizing the TTY events are sent. If the container cannot be
found, a [ContainerNotFound](#ContainerNotFound) error will be returned.
#### Example
~~~
$ varlink call -m unix:/run/io.podman/io.podman.GetAttachSockets '{"name": "b7624e775431219161"}'
{
  "sockets": {
    "container_id": "b7624e7754312191613245ce1a46844abee60025818fe3c3f3203435623a1eca",
    "control_socket": "/var/lib/containers/storage/overlay-containers/b7624e7754312191613245ce1a46844abee60025818fe3c3f3203435623a1eca/userdata/ctl",
    "io_socket": "/var/run/libpod/socket/b7624e7754312191613245ce1a46844abee60025818fe3c3f3203435623a1eca/attach"
  }
}
~~~
### <a name="GetContainer"></a>func GetContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method GetContainer(id: [string](https://godoc.org/builtin#string)) [Container](#Container)</div>
GetContainer returns information about a single container.  If a container
with the given id doesn't exist, a [ContainerNotFound](#ContainerNotFound)
error will be returned.  See also [ListContainers](ListContainers) and
[InspectContainer](#InspectContainer).
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
[ContainerNotFound](#ContainerNotFound) error will be returned. If the container is not running, a [NoContainerRunning](#NoContainerRunning)
error will be returned
#### Example
~~~
$ varlink call -m unix:/run/podman/io.podman/io.podman.GetContainerStats '{"name": "c33e4164f384"}'
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
### <a name="GetContainerStatsWithHistory"></a>func GetContainerStatsWithHistory
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method GetContainerStatsWithHistory(previousStats: [ContainerStats](#ContainerStats)) [ContainerStats](#ContainerStats)</div>
GetContainerStatsWithHistory takes a previous set of container statistics and uses libpod functions
to calculate the containers statistics based on current and previous measurements.
### <a name="GetContainersByContext"></a>func GetContainersByContext
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method GetContainersByContext(all: [bool](https://godoc.org/builtin#bool), latest: [bool](https://godoc.org/builtin#bool), args: [[]string](#[]string)) [[]string](#[]string)</div>
GetContainersByContext allows you to get a list of container ids depending on all, latest, or a list of
container names.  The definition of latest container means the latest by creation date.  In a multi-
user environment, results might differ from what you expect.
### <a name="GetContainersByStatus"></a>func GetContainersByStatus
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method GetContainersByStatus(status: [[]string](#[]string)) [Container](#Container)</div>

### <a name="GetContainersLogs"></a>func GetContainersLogs
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method GetContainersLogs(names: [[]string](#[]string), follow: [bool](https://godoc.org/builtin#bool), latest: [bool](https://godoc.org/builtin#bool), since: [string](https://godoc.org/builtin#string), tail: [int](https://godoc.org/builtin#int), timestamps: [bool](https://godoc.org/builtin#bool)) [LogLine](#LogLine)</div>

### <a name="GetEvents"></a>func GetEvents
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method GetEvents(filter: [[]string](#[]string), since: [string](https://godoc.org/builtin#string), until: [string](https://godoc.org/builtin#string)) [Event](#Event)</div>
GetEvents returns known libpod events filtered by the options provided.
### <a name="GetImage"></a>func GetImage
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method GetImage(id: [string](https://godoc.org/builtin#string)) [Image](#Image)</div>
GetImage returns information about a single image in storage.
If the image caGetImage returns be found, [ImageNotFound](#ImageNotFound) will be returned.
### <a name="GetInfo"></a>func GetInfo
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method GetInfo() [PodmanInfo](#PodmanInfo)</div>
GetInfo returns a [PodmanInfo](#PodmanInfo) struct that describes podman and its host such as storage stats,
build information of Podman, and system-wide registries.
### <a name="GetLayersMapWithImageInfo"></a>func GetLayersMapWithImageInfo
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method GetLayersMapWithImageInfo() [string](https://godoc.org/builtin#string)</div>
GetLayersMapWithImageInfo is for the development of Podman and should not be used.
### <a name="GetPod"></a>func GetPod
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method GetPod(name: [string](https://godoc.org/builtin#string)) [ListPodData](#ListPodData)</div>
GetPod takes a name or ID of a pod and returns single [ListPodData](#ListPodData)
structure.  A [PodNotFound](#PodNotFound) error will be returned if the pod cannot be found.
See also [ListPods](ListPods).
#### Example
~~~
$ varlink call -m unix:/run/podman/io.podman/io.podman.GetPod '{"name": "foobar"}'
{
  "pod": {
    "cgroup": "machine.slice",
    "containersinfo": [
      {
        "id": "00c130a45de0411f109f1a0cfea2e298df71db20fa939de5cab8b2160a36be45",
        "name": "1840835294cf-infra",
        "status": "running"
      },
      {
        "id": "49a5cce72093a5ca47c6de86f10ad7bb36391e2d89cef765f807e460865a0ec6",
        "name": "upbeat_murdock",
        "status": "running"
      }
    ],
    "createdat": "2018-12-07 13:10:15.014139258 -0600 CST",
    "id": "1840835294cf076a822e4e12ba4152411f131bd869e7f6a4e8b16df9b0ea5c7f",
    "name": "foobar",
    "numberofcontainers": "2",
    "status": "Running"
  }
}
~~~
### <a name="GetPodStats"></a>func GetPodStats
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method GetPodStats(name: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string), [ContainerStats](#ContainerStats)</div>
GetPodStats takes the name or ID of a pod and returns a pod name and slice of ContainerStats structure which
contains attributes like memory and cpu usage.  If the pod cannot be found, a [PodNotFound](#PodNotFound)
error will be returned.  If the pod has no running containers associated with it, a [NoContainerRunning](#NoContainerRunning)
error will be returned.
#### Example
~~~
$ varlink call unix:/run/podman/io.podman/io.podman.GetPodStats '{"name": "7f62b508b6f12b11d8fe02e"}'
{
  "containers": [
    {
      "block_input": 0,
      "block_output": 0,
      "cpu": 2.833470544016107524276e-08,
      "cpu_nano": 54363072,
      "id": "a64b51f805121fe2c5a3dc5112eb61d6ed139e3d1c99110360d08b58d48e4a93",
      "mem_limit": 12276146176,
      "mem_perc": 7.974359265237864966003e-03,
      "mem_usage": 978944,
      "name": "quirky_heisenberg",
      "net_input": 866,
      "net_output": 7388,
      "pids": 1,
      "system_nano": 20000000
    }
  ],
  "pod": "7f62b508b6f12b11d8fe02e0db4de6b9e43a7d7699b33a4fc0d574f6e82b4ebd"
}
~~~
### <a name="GetPodsByContext"></a>func GetPodsByContext
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method GetPodsByContext(all: [bool](https://godoc.org/builtin#bool), latest: [bool](https://godoc.org/builtin#bool), args: [[]string](#[]string)) [[]string](#[]string)</div>
GetPodsByContext allows you to get a list pod ids depending on all, latest, or a list of
pod names.  The definition of latest pod means the latest by creation date.  In a multi-
user environment, results might differ from what you expect.
### <a name="GetPodsByStatus"></a>func GetPodsByStatus
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method GetPodsByStatus(statuses: [[]string](#[]string)) [[]string](#[]string)</div>
GetPodsByStatus searches for pods whose status is included in statuses
### <a name="GetVersion"></a>func GetVersion
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method GetVersion() [string](https://godoc.org/builtin#string), [string](https://godoc.org/builtin#string), [string](https://godoc.org/builtin#string), [string](https://godoc.org/builtin#string), [string](https://godoc.org/builtin#string), [int](https://godoc.org/builtin#int)</div>
GetVersion returns version and build information of the podman service
### <a name="GetVolumes"></a>func GetVolumes
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method GetVolumes(args: [[]string](#[]string), all: [bool](https://godoc.org/builtin#bool)) [Volume](#Volume)</div>
GetVolumes gets slice of the volumes on a remote host
### <a name="HealthCheckRun"></a>func HealthCheckRun
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method HealthCheckRun(nameOrID: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
HealthCheckRun executes defined container's healthcheck command
and returns the container's health status.
### <a name="HistoryImage"></a>func HistoryImage
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method HistoryImage(name: [string](https://godoc.org/builtin#string)) [ImageHistory](#ImageHistory)</div>
HistoryImage takes the name or ID of an image and returns information about its history and layers.  The returned
history is in the form of an array of ImageHistory structures.  If the image cannot be found, an
[ImageNotFound](#ImageNotFound) error is returned.
### <a name="ImageExists"></a>func ImageExists
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ImageExists(name: [string](https://godoc.org/builtin#string)) [int](https://godoc.org/builtin#int)</div>
ImageExists talks a full or partial image ID or name and returns an int as to whether
the image exists in local storage. An int result of 0 means the image does exist in
local storage; whereas 1 indicates the image does not exists in local storage.
#### Example
~~~
$ varlink call -m unix:/run/podman/io.podman/io.podman.ImageExists '{"name": "imageddoesntexist"}'
{
  "exists": 1
}
~~~
### <a name="ImageSave"></a>func ImageSave
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ImageSave(options: [ImageSaveOptions](#ImageSaveOptions)) [MoreResponse](#MoreResponse)</div>
ImageSave allows you to save an image from the local image storage to a tarball
### <a name="ImagesPrune"></a>func ImagesPrune
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ImagesPrune(all: [bool](https://godoc.org/builtin#bool)) [[]string](#[]string)</div>
ImagesPrune removes all unused images from the local store.  Upon successful pruning,
the IDs of the removed images are returned.
### <a name="ImportImage"></a>func ImportImage
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ImportImage(source: [string](https://godoc.org/builtin#string), reference: [string](https://godoc.org/builtin#string), message: [string](https://godoc.org/builtin#string), changes: [[]string](#[]string), delete: [bool](https://godoc.org/builtin#bool)) [string](https://godoc.org/builtin#string)</div>
ImportImage imports an image from a source (like tarball) into local storage.  The image can have additional
descriptions added to it using the message and changes options. See also [ExportImage](ExportImage).
### <a name="InitContainer"></a>func InitContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method InitContainer(name: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
InitContainer initializes the given container. It accepts a container name or
ID, and will initialize the container matching that ID if possible, and error
if not. Containers can only be initialized when they are in the Created or
Exited states. Initialization prepares a container to be started, but does not
start the container. It is intended to be used to debug a container's state
prior to starting it.
### <a name="InspectContainer"></a>func InspectContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method InspectContainer(name: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
InspectContainer data takes a name or ID of a container returns the inspection
data in string format.  You can then serialize the string into JSON.  A [ContainerNotFound](#ContainerNotFound)
error will be returned if the container cannot be found. See also [InspectImage](#InspectImage).
### <a name="InspectImage"></a>func InspectImage
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method InspectImage(name: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
InspectImage takes the name or ID of an image and returns a string representation of data associated with the
mage.  You must serialize the string into JSON to use it further.  An [ImageNotFound](#ImageNotFound) error will
be returned if the image cannot be found.
### <a name="InspectPod"></a>func InspectPod
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method InspectPod(name: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
InspectPod takes the name or ID of an image and returns a string representation of data associated with the
pod.  You must serialize the string into JSON to use it further.  A [PodNotFound](#PodNotFound) error will
be returned if the pod cannot be found.
### <a name="KillContainer"></a>func KillContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method KillContainer(name: [string](https://godoc.org/builtin#string), signal: [int](https://godoc.org/builtin#int)) [string](https://godoc.org/builtin#string)</div>
KillContainer takes the name or ID of a container as well as a signal to be applied to the container.  Once the
container has been killed, the container's ID is returned.  If the container cannot be found, a
[ContainerNotFound](#ContainerNotFound) error is returned. See also [StopContainer](StopContainer).
### <a name="KillPod"></a>func KillPod
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method KillPod(name: [string](https://godoc.org/builtin#string), signal: [int](https://godoc.org/builtin#int)) [string](https://godoc.org/builtin#string)</div>
KillPod takes the name or ID of a pod as well as a signal to be applied to the pod.  If the pod cannot be found, a
[PodNotFound](#PodNotFound) error is returned.
Containers in a pod are killed independently. If there is an error killing one container, the ID of those containers
will be returned in a list, along with the ID of the pod in a [PodContainerError](#PodContainerError).
If the pod was killed with no errors, the pod ID is returned.
See also [StopPod](StopPod).
#### Example
~~~
$ varlink call -m unix:/run/podman/io.podman/io.podman.KillPod '{"name": "foobar", "signal": 15}'
{
  "pod": "1840835294cf076a822e4e12ba4152411f131bd869e7f6a4e8b16df9b0ea5c7f"
}
~~~
### <a name="ListContainerChanges"></a>func ListContainerChanges
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ListContainerChanges(name: [string](https://godoc.org/builtin#string)) [ContainerChanges](#ContainerChanges)</div>
ListContainerChanges takes a name or ID of a container and returns changes between the container and
its base image. It returns a struct of changed, deleted, and added path names.
### <a name="ListContainerMounts"></a>func ListContainerMounts
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ListContainerMounts() [map[string]](#map[string])</div>
ListContainerMounts gathers all the mounted container mount points and returns them as an array
of strings
#### Example
~~~
$ varlink call unix:/run/podman/io.podman/io.podman.ListContainerMounts
{
  "mounts": {
    "04e4c255269ed2545e7f8bd1395a75f7949c50c223415c00c1d54bfa20f3b3d9": "/var/lib/containers/storage/overlay/a078925828f57e20467ca31cfca8a849210d21ec7e5757332b72b6924f441c17/merged",
    "1d58c319f9e881a644a5122ff84419dccf6d138f744469281446ab243ef38924": "/var/lib/containers/storage/overlay/948fcf93f8cb932f0f03fd52e3180a58627d547192ffe3b88e0013b98ddcd0d2/merged"
  }
}
~~~
### <a name="ListContainerProcesses"></a>func ListContainerProcesses
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ListContainerProcesses(name: [string](https://godoc.org/builtin#string), opts: [[]string](#[]string)) [[]string](#[]string)</div>
ListContainerProcesses takes a name or ID of a container and returns the processes
running inside the container as array of strings.  It will accept an array of string
arguments that represent ps options.  If the container cannot be found, a [ContainerNotFound](#ContainerNotFound)
error will be returned.
#### Example
~~~
$ varlink call -m unix:/run/podman/io.podman/io.podman.ListContainerProcesses '{"name": "135d71b9495f", "opts": []}'
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

method ListContainers() [Container](#Container)</div>
ListContainers returns information about all containers.
See also [GetContainer](#GetContainer).
### <a name="ListImages"></a>func ListImages
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ListImages() [Image](#Image)</div>
ListImages returns information about the images that are currently in storage.
See also [InspectImage](#InspectImage).
### <a name="ListPods"></a>func ListPods
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ListPods() [ListPodData](#ListPodData)</div>
ListPods returns a list of pods in no particular order.  They are
returned as an array of ListPodData structs.  See also [GetPod](#GetPod).
#### Example
~~~
$ varlink call -m unix:/run/podman/io.podman/io.podman.ListPods
{
  "pods": [
    {
      "cgroup": "machine.slice",
      "containersinfo": [
        {
          "id": "00c130a45de0411f109f1a0cfea2e298df71db20fa939de5cab8b2160a36be45",
          "name": "1840835294cf-infra",
          "status": "running"
        },
        {
          "id": "49a5cce72093a5ca47c6de86f10ad7bb36391e2d89cef765f807e460865a0ec6",
          "name": "upbeat_murdock",
          "status": "running"
        }
      ],
      "createdat": "2018-12-07 13:10:15.014139258 -0600 CST",
      "id": "1840835294cf076a822e4e12ba4152411f131bd869e7f6a4e8b16df9b0ea5c7f",
      "name": "foobar",
      "numberofcontainers": "2",
      "status": "Running"
    },
    {
      "cgroup": "machine.slice",
      "containersinfo": [
        {
          "id": "1ca4b7bbba14a75ba00072d4b705c77f3df87db0109afaa44d50cb37c04a477e",
          "name": "784306f655c6-infra",
          "status": "running"
        }
      ],
      "createdat": "2018-12-07 13:09:57.105112457 -0600 CST",
      "id": "784306f655c6200aea321dd430ba685e9b2cc1f7d7528a72f3ff74ffb29485a2",
      "name": "nostalgic_pike",
      "numberofcontainers": "1",
      "status": "Running"
    }
  ]
}
~~~
### <a name="LoadImage"></a>func LoadImage
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method LoadImage(name: [string](https://godoc.org/builtin#string), inputFile: [string](https://godoc.org/builtin#string), quiet: [bool](https://godoc.org/builtin#bool), deleteFile: [bool](https://godoc.org/builtin#bool)) [MoreResponse](#MoreResponse)</div>
LoadImage allows you to load an image into local storage from a tarball.
### <a name="MountContainer"></a>func MountContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method MountContainer(name: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
MountContainer mounts a container by name or full/partial ID.  Upon a successful mount, the destination
mount is returned as a string.
#### Example
~~~
$ varlink call -m unix:/run/podman/io.podman/io.podman.MountContainer '{"name": "jolly_shannon"}'{
  "path": "/var/lib/containers/storage/overlay/419eeb04e783ea159149ced67d9fcfc15211084d65e894792a96bedfae0470ca/merged"
}
~~~
### <a name="PauseContainer"></a>func PauseContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method PauseContainer(name: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
PauseContainer takes the name or ID of container and pauses it.  If the container cannot be found,
a [ContainerNotFound](#ContainerNotFound) error will be returned; otherwise the ID of the container is returned.
See also [UnpauseContainer](#UnpauseContainer).
### <a name="PausePod"></a>func PausePod
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method PausePod(name: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
PausePod takes the name or ID of a pod and pauses the running containers associated with it.  If the pod cannot be found,
a [PodNotFound](#PodNotFound) error will be returned.
Containers in a pod are paused independently. If there is an error pausing one container, the ID of those containers
will be returned in a list, along with the ID of the pod in a [PodContainerError](#PodContainerError).
If the pod was paused with no errors, the pod ID is returned.
See also [UnpausePod](#UnpausePod).
#### Example
~~~
$ varlink call -m unix:/run/podman/io.podman/io.podman.PausePod '{"name": "foobar"}'
{
  "pod": "1840835294cf076a822e4e12ba4152411f131bd869e7f6a4e8b16df9b0ea5c7f"
}
~~~
### <a name="PodStateData"></a>func PodStateData
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method PodStateData(name: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
PodStateData returns inspectr level information of a given pod in string form.  This call is for
development of Podman only and generally should not be used.
### <a name="Ps"></a>func Ps
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method Ps(opts: [PsOpts](#PsOpts)) [PsContainer](#PsContainer)</div>

### <a name="PullImage"></a>func PullImage
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method PullImage(name: [string](https://godoc.org/builtin#string)) [MoreResponse](#MoreResponse)</div>
PullImage pulls an image from a repository to local storage.  After a successful pull, the image id and logs
are returned as a [MoreResponse](#MoreResponse).  This connection also will handle a WantsMores request to send
status as it occurs.
### <a name="PushImage"></a>func PushImage
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method PushImage(name: [string](https://godoc.org/builtin#string), tag: [string](https://godoc.org/builtin#string), compress: [bool](https://godoc.org/builtin#bool), format: [string](https://godoc.org/builtin#string), removeSignatures: [bool](https://godoc.org/builtin#bool), signBy: [string](https://godoc.org/builtin#string)) [MoreResponse](#MoreResponse)</div>
PushImage takes two input arguments: the name or ID of an image, the fully-qualified destination name of the image,
It will return an [ImageNotFound](#ImageNotFound) error if
the image cannot be found in local storage; otherwise it will return a [MoreResponse](#MoreResponse)
### <a name="ReceiveFile"></a>func ReceiveFile
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method ReceiveFile(path: [string](https://godoc.org/builtin#string), delete: [bool](https://godoc.org/builtin#bool)) [int](https://godoc.org/builtin#int)</div>
ReceiveFile allows the host to send a remote client a file
### <a name="RemoveContainer"></a>func RemoveContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method RemoveContainer(name: [string](https://godoc.org/builtin#string), force: [bool](https://godoc.org/builtin#bool), removeVolumes: [bool](https://godoc.org/builtin#bool)) [string](https://godoc.org/builtin#string)</div>
RemoveContainer requires the name or ID of a container as well as a boolean that
indicates whether a container should be forcefully removed (e.g., by stopping it), and a boolean
indicating whether to remove builtin volumes. Upon successful removal of the
container, its ID is returned.  If the
container cannot be found by name or ID, a [ContainerNotFound](#ContainerNotFound) error will be returned.
See also [EvictContainer](EvictContainer).
#### Example
~~~
$ varlink call -m unix:/run/podman/io.podman/io.podman.RemoveContainer '{"name": "62f4fd98cb57"}'
{
  "container": "62f4fd98cb57f529831e8f90610e54bba74bd6f02920ffb485e15376ed365c20"
}
~~~
### <a name="RemoveImage"></a>func RemoveImage
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method RemoveImage(name: [string](https://godoc.org/builtin#string), force: [bool](https://godoc.org/builtin#bool)) [string](https://godoc.org/builtin#string)</div>
RemoveImage takes the name or ID of an image as well as a boolean that determines if containers using that image
should be deleted.  If the image cannot be found, an [ImageNotFound](#ImageNotFound) error will be returned.  The
ID of the removed image is returned when complete.  See also [DeleteUnusedImages](DeleteUnusedImages).
#### Example
~~~
varlink call -m unix:/run/podman/io.podman/io.podman.RemoveImage '{"name": "registry.fedoraproject.org/fedora", "force": true}'
{
  "image": "426866d6fa419873f97e5cbd320eeb22778244c1dfffa01c944db3114f55772e"
}
~~~
### <a name="RemovePod"></a>func RemovePod
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method RemovePod(name: [string](https://godoc.org/builtin#string), force: [bool](https://godoc.org/builtin#bool)) [string](https://godoc.org/builtin#string)</div>
RemovePod takes the name or ID of a pod as well a boolean representing whether a running
container in the pod can be stopped and removed.  If a pod has containers associated with it, and force is not true,
an error will occur.
If the pod cannot be found by name or ID, a [PodNotFound](#PodNotFound) error will be returned.
Containers in a pod are removed independently. If there is an error removing any container, the ID of those containers
will be returned in a list, along with the ID of the pod in a [PodContainerError](#PodContainerError).
If the pod was removed with no errors, the pod ID is returned.
#### Example
~~~
$ varlink call -m unix:/run/podman/io.podman/io.podman.RemovePod '{"name": "62f4fd98cb57", "force": "true"}'
{
  "pod": "62f4fd98cb57f529831e8f90610e54bba74bd6f02920ffb485e15376ed365c20"
}
~~~
### <a name="RestartContainer"></a>func RestartContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method RestartContainer(name: [string](https://godoc.org/builtin#string), timeout: [int](https://godoc.org/builtin#int)) [string](https://godoc.org/builtin#string)</div>
RestartContainer will restart a running container given a container name or ID and timeout value. The timeout
value is the time before a forcible stop is used to stop the container.  If the container cannot be found by
name or ID, a [ContainerNotFound](#ContainerNotFound)  error will be returned; otherwise, the ID of the
container will be returned.
### <a name="RestartPod"></a>func RestartPod
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method RestartPod(name: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
RestartPod will restart containers in a pod given a pod name or ID. Containers in
the pod that are running will be stopped, then all stopped containers will be run.
If the pod cannot be found by name or ID, a [PodNotFound](#PodNotFound) error will be returned.
Containers in a pod are restarted independently. If there is an error restarting one container, the ID of those containers
will be returned in a list, along with the ID of the pod in a [PodContainerError](#PodContainerError).
If the pod was restarted with no errors, the pod ID is returned.
#### Example
~~~
$ varlink call -m unix:/run/podman/io.podman/io.podman.RestartPod '{"name": "135d71b9495f"}'
{
  "pod": "135d71b9495f7c3967f536edad57750bfdb569336cd107d8aabab45565ffcfb6"
}
~~~
### <a name="SearchImages"></a>func SearchImages
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method SearchImages(query: [string](https://godoc.org/builtin#string), limit: [?int](#?int), filter: [ImageSearchFilter](#ImageSearchFilter)) [ImageSearchResult](#ImageSearchResult)</div>
SearchImages searches available registries for images that contain the
contents of "query" in their name. If "limit" is given, limits the amount of
search results per registry.
### <a name="SendFile"></a>func SendFile
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method SendFile(type: [string](https://godoc.org/builtin#string), length: [int](https://godoc.org/builtin#int)) [string](https://godoc.org/builtin#string)</div>
Sendfile allows a remote client to send a file to the host
### <a name="Spec"></a>func Spec
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method Spec(name: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
Spec returns the oci spec for a container.  This call is for development of Podman only and generally should not be used.
### <a name="StartContainer"></a>func StartContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method StartContainer(name: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
StartContainer starts a created or stopped container. It takes the name or ID of container.  It returns
the container ID once started.  If the container cannot be found, a [ContainerNotFound](#ContainerNotFound)
error will be returned.  See also [CreateContainer](#CreateContainer).
### <a name="StartPod"></a>func StartPod
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method StartPod(name: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
StartPod starts containers in a pod.  It takes the name or ID of pod.  If the pod cannot be found, a [PodNotFound](#PodNotFound)
error will be returned.  Containers in a pod are started independently. If there is an error starting one container, the ID of those containers
will be returned in a list, along with the ID of the pod in a [PodContainerError](#PodContainerError).
If the pod was started with no errors, the pod ID is returned.
See also [CreatePod](#CreatePod).
#### Example
~~~
$ varlink call -m unix:/run/podman/io.podman/io.podman.StartPod '{"name": "135d71b9495f"}'
{
  "pod": "135d71b9495f7c3967f536edad57750bfdb569336cd107d8aabab45565ffcfb6",
}
~~~
### <a name="StopContainer"></a>func StopContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method StopContainer(name: [string](https://godoc.org/builtin#string), timeout: [int](https://godoc.org/builtin#int)) [string](https://godoc.org/builtin#string)</div>
StopContainer stops a container given a timeout.  It takes the name or ID of a container as well as a
timeout value.  The timeout value the time before a forcible stop to the container is applied.  It
returns the container ID once stopped. If the container cannot be found, a [ContainerNotFound](#ContainerNotFound)
error will be returned instead. See also [KillContainer](KillContainer).
#### Error
~~~
$ varlink call -m unix:/run/podman/io.podman/io.podman.StopContainer '{"name": "135d71b9495f", "timeout": 5}'
{
  "container": "135d71b9495f7c3967f536edad57750bfdb569336cd107d8aabab45565ffcfb6"
}
~~~
### <a name="StopPod"></a>func StopPod
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method StopPod(name: [string](https://godoc.org/builtin#string), timeout: [int](https://godoc.org/builtin#int)) [string](https://godoc.org/builtin#string)</div>
StopPod stops containers in a pod.  It takes the name or ID of a pod and a timeout.
If the pod cannot be found, a [PodNotFound](#PodNotFound) error will be returned instead.
Containers in a pod are stopped independently. If there is an error stopping one container, the ID of those containers
will be returned in a list, along with the ID of the pod in a [PodContainerError](#PodContainerError).
If the pod was stopped with no errors, the pod ID is returned.
See also [KillPod](KillPod).
#### Example
~~~
$ varlink call -m unix:/run/podman/io.podman/io.podman.StopPod '{"name": "135d71b9495f"}'
{
  "pod": "135d71b9495f7c3967f536edad57750bfdb569336cd107d8aabab45565ffcfb6"
}
~~~
### <a name="TagImage"></a>func TagImage
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method TagImage(name: [string](https://godoc.org/builtin#string), tagged: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
TagImage takes the name or ID of an image in local storage as well as the desired tag name.  If the image cannot
be found, an [ImageNotFound](#ImageNotFound) error will be returned; otherwise, the ID of the image is returned on success.
### <a name="Top"></a>func Top
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method Top(nameOrID: [string](https://godoc.org/builtin#string), descriptors: [[]string](#[]string)) [[]string](#[]string)</div>

### <a name="TopPod"></a>func TopPod
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method TopPod(pod: [string](https://godoc.org/builtin#string), latest: [bool](https://godoc.org/builtin#bool), descriptors: [[]string](#[]string)) [[]string](#[]string)</div>

### <a name="UnmountContainer"></a>func UnmountContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method UnmountContainer(name: [string](https://godoc.org/builtin#string), force: [bool](https://godoc.org/builtin#bool)) </div>
UnmountContainer umounts a container by its name or full/partial container ID.
#### Example
~~~
$ varlink call -m unix:/run/podman/io.podman/io.podman.UnmountContainer '{"name": "jolly_shannon", "force": false}'
{}
~~~
### <a name="UnpauseContainer"></a>func UnpauseContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method UnpauseContainer(name: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
UnpauseContainer takes the name or ID of container and unpauses a paused container.  If the container cannot be
found, a [ContainerNotFound](#ContainerNotFound) error will be returned; otherwise the ID of the container is returned.
See also [PauseContainer](#PauseContainer).
### <a name="UnpausePod"></a>func UnpausePod
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method UnpausePod(name: [string](https://godoc.org/builtin#string)) [string](https://godoc.org/builtin#string)</div>
UnpausePod takes the name or ID of a pod and unpauses the paused containers associated with it.  If the pod cannot be
found, a [PodNotFound](#PodNotFound) error will be returned.
Containers in a pod are unpaused independently. If there is an error unpausing one container, the ID of those containers
will be returned in a list, along with the ID of the pod in a [PodContainerError](#PodContainerError).
If the pod was unpaused with no errors, the pod ID is returned.
See also [PausePod](#PausePod).
#### Example
~~~
$ varlink call -m unix:/run/podman/io.podman/io.podman.UnpausePod '{"name": "foobar"}'
{
  "pod": "1840835294cf076a822e4e12ba4152411f131bd869e7f6a4e8b16df9b0ea5c7f"
}
~~~
### <a name="VolumeCreate"></a>func VolumeCreate
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method VolumeCreate(options: [VolumeCreateOpts](#VolumeCreateOpts)) [string](https://godoc.org/builtin#string)</div>
VolumeCreate creates a volume on a remote host
### <a name="VolumeRemove"></a>func VolumeRemove
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method VolumeRemove(options: [VolumeRemoveOpts](#VolumeRemoveOpts)) [[]string](#[]string), [map[string]](#map[string])</div>
VolumeRemove removes a volume on a remote host
### <a name="VolumesPrune"></a>func VolumesPrune
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method VolumesPrune() [[]string](#[]string), [[]string](#[]string)</div>
VolumesPrune removes unused volumes on the host
### <a name="WaitContainer"></a>func WaitContainer
<div style="background-color: #E8E8E8; padding: 15px; margin: 10px; border-radius: 10px;">

method WaitContainer(name: [string](https://godoc.org/builtin#string), interval: [int](https://godoc.org/builtin#int)) [int](https://godoc.org/builtin#int)</div>
WaitContainer takes the name or ID of a container and waits the given interval in milliseconds until the container
stops.  Upon stopping, the return code of the container is returned. If the container container cannot be found by ID
or name, a [ContainerNotFound](#ContainerNotFound) error is returned.
## Types
### <a name="BuildInfo"></a>type BuildInfo

BuildInfo is used to describe user input for building images

additionalTags [[]string](#[]string)

annotations [[]string](#[]string)

buildArgs [map[string]](#map[string])

buildOptions [BuildOptions](#BuildOptions)

cniConfigDir [string](https://godoc.org/builtin#string)

cniPluginDir [string](https://godoc.org/builtin#string)

compression [string](https://godoc.org/builtin#string)

contextDir [string](https://godoc.org/builtin#string)

defaultsMountFilePath [string](https://godoc.org/builtin#string)

dockerfiles [[]string](#[]string)

err [string](https://godoc.org/builtin#string)

forceRmIntermediateCtrs [bool](https://godoc.org/builtin#bool)

iidfile [string](https://godoc.org/builtin#string)

label [[]string](#[]string)

layers [bool](https://godoc.org/builtin#bool)

nocache [bool](https://godoc.org/builtin#bool)

out [string](https://godoc.org/builtin#string)

output [string](https://godoc.org/builtin#string)

outputFormat [string](https://godoc.org/builtin#string)

pullPolicy [string](https://godoc.org/builtin#string)

quiet [bool](https://godoc.org/builtin#bool)

remoteIntermediateCtrs [bool](https://godoc.org/builtin#bool)

reportWriter [string](https://godoc.org/builtin#string)

runtimeArgs [[]string](#[]string)

squash [bool](https://godoc.org/builtin#bool)
### <a name="BuildOptions"></a>type BuildOptions

BuildOptions are are used to describe describe physical attributes of the build

addHosts [[]string](#[]string)

cgroupParent [string](https://godoc.org/builtin#string)

cpuPeriod [int](https://godoc.org/builtin#int)

cpuQuota [int](https://godoc.org/builtin#int)

cpuShares [int](https://godoc.org/builtin#int)

cpusetCpus [string](https://godoc.org/builtin#string)

cpusetMems [string](https://godoc.org/builtin#string)

memory [int](https://godoc.org/builtin#int)

memorySwap [int](https://godoc.org/builtin#int)

shmSize [string](https://godoc.org/builtin#string)

ulimit [[]string](#[]string)

volume [[]string](#[]string)
### <a name="Container"></a>type Container



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
### <a name="Create"></a>type Create

Create is an input structure for creating containers.
args[0] is the image name or id
args[1-] are the new commands if changed

args [[]string](#[]string)

addHost [?[]string](#?[]string)

annotation [?[]string](#?[]string)

attach [?[]string](#?[]string)

blkioWeight [?string](#?string)

blkioWeightDevice [?[]string](#?[]string)

capAdd [?[]string](#?[]string)

capDrop [?[]string](#?[]string)

cgroupParent [?string](#?string)

cidFile [?string](#?string)

conmonPidfile [?string](#?string)

command [?[]string](#?[]string)

cpuPeriod [?int](#?int)

cpuQuota [?int](#?int)

cpuRtPeriod [?int](#?int)

cpuRtRuntime [?int](#?int)

cpuShares [?int](#?int)

cpus [?float](#?float)

cpuSetCpus [?string](#?string)

cpuSetMems [?string](#?string)

detach [?bool](#?bool)

detachKeys [?string](#?string)

device [?[]string](#?[]string)

deviceReadBps [?[]string](#?[]string)

deviceReadIops [?[]string](#?[]string)

deviceWriteBps [?[]string](#?[]string)

deviceWriteIops [?[]string](#?[]string)

dns [?[]string](#?[]string)

dnsOpt [?[]string](#?[]string)

dnsSearch [?[]string](#?[]string)

dnsServers [?[]string](#?[]string)

entrypoint [?string](#?string)

env [?[]string](#?[]string)

envFile [?[]string](#?[]string)

expose [?[]string](#?[]string)

gidmap [?[]string](#?[]string)

groupadd [?[]string](#?[]string)

healthcheckCommand [?string](#?string)

healthcheckInterval [?string](#?string)

healthcheckRetries [?int](#?int)

healthcheckStartPeriod [?string](#?string)

healthcheckTimeout [?string](#?string)

hostname [?string](#?string)

imageVolume [?string](#?string)

init [?bool](#?bool)

initPath [?string](#?string)

interactive [?bool](#?bool)

ip [?string](#?string)

ipc [?string](#?string)

kernelMemory [?string](#?string)

label [?[]string](#?[]string)

labelFile [?[]string](#?[]string)

logDriver [?string](#?string)

logOpt [?[]string](#?[]string)

macAddress [?string](#?string)

memory [?string](#?string)

memoryReservation [?string](#?string)

memorySwap [?string](#?string)

memorySwappiness [?int](#?int)

name [?string](#?string)

net [?string](#?string)

network [?string](#?string)

noHosts [?bool](#?bool)

oomKillDisable [?bool](#?bool)

oomScoreAdj [?int](#?int)

pid [?string](#?string)

pidsLimit [?int](#?int)

pod [?string](#?string)

privileged [?bool](#?bool)

publish [?[]string](#?[]string)

publishAll [?bool](#?bool)

pull [?string](#?string)

quiet [?bool](#?bool)

readonly [?bool](#?bool)

readonlytmpfs [?bool](#?bool)

restart [?string](#?string)

rm [?bool](#?bool)

rootfs [?bool](#?bool)

securityOpt [?[]string](#?[]string)

shmSize [?string](#?string)

stopSignal [?string](#?string)

stopTimeout [?int](#?int)

storageOpt [?[]string](#?[]string)

subuidname [?string](#?string)

subgidname [?string](#?string)

sysctl [?[]string](#?[]string)

systemd [?bool](#?bool)

tmpfs [?[]string](#?[]string)

tty [?bool](#?bool)

uidmap [?[]string](#?[]string)

ulimit [?[]string](#?[]string)

user [?string](#?string)

userns [?string](#?string)

uts [?string](#?string)

mount [?[]string](#?[]string)

volume [?[]string](#?[]string)

volumesFrom [?[]string](#?[]string)

workDir [?string](#?string)
### <a name="DiffInfo"></a>type DiffInfo



path [string](https://godoc.org/builtin#string)

changeType [string](https://godoc.org/builtin#string)
### <a name="Event"></a>type Event

Event describes a libpod struct

id [string](https://godoc.org/builtin#string)

image [string](https://godoc.org/builtin#string)

name [string](https://godoc.org/builtin#string)

status [string](https://godoc.org/builtin#string)

time [string](https://godoc.org/builtin#string)

type [string](https://godoc.org/builtin#string)
### <a name="ExecOpts"></a>type ExecOpts



name [string](https://godoc.org/builtin#string)

tty [bool](https://godoc.org/builtin#bool)

privileged [bool](https://godoc.org/builtin#bool)

cmd [[]string](#[]string)

user [?string](#?string)

workdir [?string](#?string)

env [?[]string](#?[]string)

detachKeys [?string](#?string)
### <a name="Image"></a>type Image



id [string](https://godoc.org/builtin#string)

digest [string](https://godoc.org/builtin#string)

parentId [string](https://godoc.org/builtin#string)

repoTags [[]string](#[]string)

repoDigests [[]string](#[]string)

created [string](https://godoc.org/builtin#string)

size [int](https://godoc.org/builtin#int)

virtualSize [int](https://godoc.org/builtin#int)

containers [int](https://godoc.org/builtin#int)

labels [map[string]](#map[string])

isParent [bool](https://godoc.org/builtin#bool)

topLayer [string](https://godoc.org/builtin#string)

readOnly [bool](https://godoc.org/builtin#bool)
### <a name="ImageHistory"></a>type ImageHistory

ImageHistory describes the returned structure from ImageHistory.

id [string](https://godoc.org/builtin#string)

created [string](https://godoc.org/builtin#string)

createdBy [string](https://godoc.org/builtin#string)

tags [[]string](#[]string)

size [int](https://godoc.org/builtin#int)

comment [string](https://godoc.org/builtin#string)
### <a name="ImageSaveOptions"></a>type ImageSaveOptions



name [string](https://godoc.org/builtin#string)

format [string](https://godoc.org/builtin#string)

output [string](https://godoc.org/builtin#string)

outputType [string](https://godoc.org/builtin#string)

moreTags [[]string](#[]string)

quiet [bool](https://godoc.org/builtin#bool)

compress [bool](https://godoc.org/builtin#bool)
### <a name="ImageSearchFilter"></a>type ImageSearchFilter



is_official [?bool](#?bool)

is_automated [?bool](#?bool)

star_count [int](https://godoc.org/builtin#int)
### <a name="ImageSearchResult"></a>type ImageSearchResult

Represents a single search result from SearchImages

description [string](https://godoc.org/builtin#string)

is_official [bool](https://godoc.org/builtin#bool)

is_automated [bool](https://godoc.org/builtin#bool)

registry [string](https://godoc.org/builtin#string)

name [string](https://godoc.org/builtin#string)

star_count [int](https://godoc.org/builtin#int)
### <a name="InfoDistribution"></a>type InfoDistribution

InfoDistribution describes the host's distribution

distribution [string](https://godoc.org/builtin#string)

version [string](https://godoc.org/builtin#string)
### <a name="InfoGraphStatus"></a>type InfoGraphStatus

InfoGraphStatus describes the detailed status of the storage driver

backing_filesystem [string](https://godoc.org/builtin#string)

native_overlay_diff [string](https://godoc.org/builtin#string)

supports_d_type [string](https://godoc.org/builtin#string)
### <a name="InfoHost"></a>type InfoHost

InfoHost describes the host stats portion of PodmanInfo

buildah_version [string](https://godoc.org/builtin#string)

distribution [InfoDistribution](#InfoDistribution)

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

eventlogger [string](https://godoc.org/builtin#string)
### <a name="InfoPodmanBinary"></a>type InfoPodmanBinary

InfoPodman provides details on the Podman binary

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
### <a name="KubePodService"></a>type KubePodService



pod [string](https://godoc.org/builtin#string)

service [string](https://godoc.org/builtin#string)
### <a name="ListPodContainerInfo"></a>type ListPodContainerInfo

ListPodContainerInfo is a returned struct for describing containers
in a pod.

name [string](https://godoc.org/builtin#string)

id [string](https://godoc.org/builtin#string)

status [string](https://godoc.org/builtin#string)
### <a name="ListPodData"></a>type ListPodData

ListPodData is the returned struct for an individual pod

id [string](https://godoc.org/builtin#string)

name [string](https://godoc.org/builtin#string)

createdat [string](https://godoc.org/builtin#string)

cgroup [string](https://godoc.org/builtin#string)

status [string](https://godoc.org/builtin#string)

labels [map[string]](#map[string])

numberofcontainers [string](https://godoc.org/builtin#string)

containersinfo [ListPodContainerInfo](#ListPodContainerInfo)
### <a name="LogLine"></a>type LogLine



device [string](https://godoc.org/builtin#string)

parseLogType [string](https://godoc.org/builtin#string)

time [string](https://godoc.org/builtin#string)

msg [string](https://godoc.org/builtin#string)

cid [string](https://godoc.org/builtin#string)
### <a name="MoreResponse"></a>type MoreResponse

MoreResponse is a struct for when responses from varlink requires longer output

logs [[]string](#[]string)

id [string](https://godoc.org/builtin#string)
### <a name="NotImplemented"></a>type NotImplemented



comment [string](https://godoc.org/builtin#string)
### <a name="PodContainerErrorData"></a>type PodContainerErrorData



containerid [string](https://godoc.org/builtin#string)

reason [string](https://godoc.org/builtin#string)
### <a name="PodCreate"></a>type PodCreate

PodCreate is an input structure for creating pods.
It emulates options to podman pod create. The infraCommand and
infraImage options are currently NotSupported.

name [string](https://godoc.org/builtin#string)

cgroupParent [string](https://godoc.org/builtin#string)

labels [map[string]](#map[string])

share [[]string](#[]string)

infra [bool](https://godoc.org/builtin#bool)

infraCommand [string](https://godoc.org/builtin#string)

infraImage [string](https://godoc.org/builtin#string)

publish [[]string](#[]string)
### <a name="PodmanInfo"></a>type PodmanInfo

PodmanInfo describes the Podman host and build

host [InfoHost](#InfoHost)

registries [[]string](#[]string)

insecure_registries [[]string](#[]string)

store [InfoStore](#InfoStore)

podman [InfoPodmanBinary](#InfoPodmanBinary)
### <a name="PsContainer"></a>type PsContainer



id [string](https://godoc.org/builtin#string)

image [string](https://godoc.org/builtin#string)

command [string](https://godoc.org/builtin#string)

created [string](https://godoc.org/builtin#string)

ports [string](https://godoc.org/builtin#string)

names [string](https://godoc.org/builtin#string)

isInfra [bool](https://godoc.org/builtin#bool)

status [string](https://godoc.org/builtin#string)

state [string](https://godoc.org/builtin#string)

pidNum [int](https://godoc.org/builtin#int)

rootFsSize [int](https://godoc.org/builtin#int)

rwSize [int](https://godoc.org/builtin#int)

pod [string](https://godoc.org/builtin#string)

createdAt [string](https://godoc.org/builtin#string)

exitedAt [string](https://godoc.org/builtin#string)

startedAt [string](https://godoc.org/builtin#string)

labels [map[string]](#map[string])

nsPid [string](https://godoc.org/builtin#string)

cgroup [string](https://godoc.org/builtin#string)

ipc [string](https://godoc.org/builtin#string)

mnt [string](https://godoc.org/builtin#string)

net [string](https://godoc.org/builtin#string)

pidNs [string](https://godoc.org/builtin#string)

user [string](https://godoc.org/builtin#string)

uts [string](https://godoc.org/builtin#string)

mounts [string](https://godoc.org/builtin#string)
### <a name="PsOpts"></a>type PsOpts



all [bool](https://godoc.org/builtin#bool)

filters [?[]string](#?[]string)

last [?int](#?int)

latest [?bool](#?bool)

noTrunc [?bool](#?bool)

pod [?bool](#?bool)

quiet [?bool](#?bool)

size [?bool](#?bool)

sort [?string](#?string)

sync [?bool](#?bool)
### <a name="Runlabel"></a>type Runlabel

Runlabel describes the required input for container runlabel

image [string](https://godoc.org/builtin#string)

authfile [string](https://godoc.org/builtin#string)

display [bool](https://godoc.org/builtin#bool)

name [string](https://godoc.org/builtin#string)

pull [bool](https://godoc.org/builtin#bool)

label [string](https://godoc.org/builtin#string)

extraArgs [[]string](#[]string)

opts [map[string]](#map[string])
### <a name="Sockets"></a>type Sockets

Sockets describes sockets location for a container

container_id [string](https://godoc.org/builtin#string)

io_socket [string](https://godoc.org/builtin#string)

control_socket [string](https://godoc.org/builtin#string)
### <a name="StringResponse"></a>type StringResponse



message [string](https://godoc.org/builtin#string)
### <a name="Volume"></a>type Volume



name [string](https://godoc.org/builtin#string)

labels [map[string]](#map[string])

mountPoint [string](https://godoc.org/builtin#string)

driver [string](https://godoc.org/builtin#string)

options [map[string]](#map[string])
### <a name="VolumeCreateOpts"></a>type VolumeCreateOpts



volumeName [string](https://godoc.org/builtin#string)

driver [string](https://godoc.org/builtin#string)

labels [map[string]](#map[string])

options [map[string]](#map[string])
### <a name="VolumeRemoveOpts"></a>type VolumeRemoveOpts



volumes [[]string](#[]string)

all [bool](https://godoc.org/builtin#bool)

force [bool](https://godoc.org/builtin#bool)
## Errors
### <a name="ContainerNotFound"></a>type ContainerNotFound

ContainerNotFound means the container could not be found by the provided name or ID in local storage.
### <a name="ErrCtrStopped"></a>type ErrCtrStopped

Container is already stopped
### <a name="ErrRequiresCgroupsV2ForRootless"></a>type ErrRequiresCgroupsV2ForRootless

This function requires CGroupsV2 to run in rootless mode.
### <a name="ErrorOccurred"></a>type ErrorOccurred

ErrorOccurred is a generic error for an error that occurs during the execution.  The actual error message
is includes as part of the error's text.
### <a name="ImageNotFound"></a>type ImageNotFound

ImageNotFound means the image could not be found by the provided name or ID in local storage.
### <a name="InvalidState"></a>type InvalidState

InvalidState indicates that a container or pod was in an improper state for the requested operation
### <a name="NoContainerRunning"></a>type NoContainerRunning

NoContainerRunning means none of the containers requested are running in a command that requires a running container.
### <a name="NoContainersInPod"></a>type NoContainersInPod

NoContainersInPod means a pod has no containers on which to perform the operation. It contains
the pod ID.
### <a name="PodContainerError"></a>type PodContainerError

PodContainerError means a container associated with a pod failed to perform an operation. It contains
a container ID of the container that failed.
### <a name="PodNotFound"></a>type PodNotFound

PodNotFound means the pod could not be found by the provided name or ID in local storage.
### <a name="RuntimeError"></a>type RuntimeError

RuntimeErrors generally means a runtime could not be found or gotten.
### <a name="VolumeNotFound"></a>type VolumeNotFound

VolumeNotFound means the volume could not be found by the name or ID in local storage.
### <a name="WantsMoreRequired"></a>type WantsMoreRequired

The Podman endpoint requires that you use a streaming connection.
