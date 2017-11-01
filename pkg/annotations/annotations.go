package annotations

const (
	// Annotations carries the received Kubelet annotations
	Annotations = "io.kubernetes.cri-o.Annotations"

	// ContainerID is the container ID annotation
	ContainerID = "io.kubernetes.cri-o.ContainerID"

	// ContainerName is the container name annotation
	ContainerName = "io.kubernetes.cri-o.ContainerName"

	// ContainerType is the container type (sandbox or container) annotation
	ContainerType = "io.kubernetes.cri-o.ContainerType"

	// Created is the container creation time annotation
	Created = "io.kubernetes.cri-o.Created"

	// HostName is the container host name annotation
	HostName = "io.kubernetes.cri-o.HostName"

	// IP is the container ipv4 or ipv6 address
	IP = "io.kubernetes.cri-o.IP"

	// Image is the container image ID annotation
	Image = "io.kubernetes.cri-o.Image"

	// ImageName is the container image name annotation
	ImageName = "io.kubernetes.cri-o.ImageName"

	// ImageRef is the container image ref annotation
	ImageRef = "io.kubernetes.cri-o.ImageRef"

	// KubeName is the kubernetes name annotation
	KubeName = "io.kubernetes.cri-o.KubeName"

	// Labels are the kubernetes labels annotation
	Labels = "io.kubernetes.cri-o.Labels"

	// LogPath is the container logging path annotation
	LogPath = "io.kubernetes.cri-o.LogPath"

	// Metadata is the container metadata annotation
	Metadata = "io.kubernetes.cri-o.Metadata"

	// Name is the pod name annotation
	Name = "io.kubernetes.cri-o.Name"

	// PrivilegedRuntime is the annotation for the privileged runtime path
	PrivilegedRuntime = "io.kubernetes.cri-o.PrivilegedRuntime"

	// ResolvPath is the resolver configuration path annotation
	ResolvPath = "io.kubernetes.cri-o.ResolvPath"

	// HostnamePath is the path to /etc/hostname to bind mount annotation
	HostnamePath = "io.kubernetes.cri-o.HostnamePath"

	// SandboxID is the sandbox ID annotation
	SandboxID = "io.kubernetes.cri-o.SandboxID"

	// SandboxName is the sandbox name annotation
	SandboxName = "io.kubernetes.cri-o.SandboxName"

	// ShmPath is the shared memory path annotation
	ShmPath = "io.kubernetes.cri-o.ShmPath"

	// MountPoint is the mount point of the container rootfs
	MountPoint = "io.kubernetes.cri-o.MountPoint"

	// TrustedSandbox is the annotation for trusted sandboxes
	TrustedSandbox = "io.kubernetes.cri-o.TrustedSandbox"

	// TTY is the terminal path annotation
	TTY = "io.kubernetes.cri-o.TTY"

	// Stdin is the stdin annotation
	Stdin = "io.kubernetes.cri-o.Stdin"

	// StdinOnce is the stdin_once annotation
	StdinOnce = "io.kubernetes.cri-o.StdinOnce"

	// Volumes is the volumes annotatoin
	Volumes = "io.kubernetes.cri-o.Volumes"
)

// ContainerType values
const (
	// ContainerTypeSandbox represents a pod sandbox container
	ContainerTypeSandbox = "sandbox"

	// ContainerTypeContainer represents a container running within a pod
	ContainerTypeContainer = "container"
)
