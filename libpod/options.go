package libpod

import (
	"fmt"
	"path/filepath"

	"github.com/containers/storage"
	"github.com/containers/storage/pkg/idtools"
	"github.com/pkg/errors"
)

var (
	ctrNotImplemented = func(c *Container) error {
		return fmt.Errorf("NOT IMPLEMENTED")
	}
)

const (
	// IPCNamespace represents the IPC namespace
	IPCNamespace = "ipc"
	// MountNamespace represents the mount namespace
	MountNamespace = "mount"
	// NetNamespace represents the network namespace
	NetNamespace = "network"
	// PIDNamespace represents the PID namespace
	PIDNamespace = "pid"
	// UserNamespace represents the user namespace
	UserNamespace = "user"
	// UTSNamespace represents the UTS namespace
	UTSNamespace = "uts"
)

// Runtime Creation Options

// WithStorageConfig uses the given configuration to set up container storage
// If this is not specified, the system default configuration will be used
// instead
func WithStorageConfig(config storage.StoreOptions) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.StorageConfig.RunRoot = config.RunRoot
		rt.config.StorageConfig.GraphRoot = config.GraphRoot
		rt.config.StorageConfig.GraphDriverName = config.GraphDriverName
		rt.config.StaticDir = filepath.Join(config.GraphRoot, "libpod")

		rt.config.StorageConfig.GraphDriverOptions = make([]string, len(config.GraphDriverOptions))
		copy(rt.config.StorageConfig.GraphDriverOptions, config.GraphDriverOptions)

		rt.config.StorageConfig.UIDMap = make([]idtools.IDMap, len(config.UIDMap))
		copy(rt.config.StorageConfig.UIDMap, config.UIDMap)

		rt.config.StorageConfig.GIDMap = make([]idtools.IDMap, len(config.GIDMap))
		copy(rt.config.StorageConfig.GIDMap, config.GIDMap)

		return nil
	}
}

// WithImageConfig uses the given configuration to set up image handling
// If this is not specified, the system default configuration will be used
// instead
func WithImageConfig(defaultTransport string, insecureRegistries, registries []string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.ImageDefaultTransport = defaultTransport

		rt.config.InsecureRegistries = make([]string, len(insecureRegistries))
		copy(rt.config.InsecureRegistries, insecureRegistries)

		rt.config.Registries = make([]string, len(registries))
		copy(rt.config.Registries, registries)

		return nil
	}
}

// WithSignaturePolicy specifies the path of a file which decides how trust is
// managed for images we've pulled.
// If this is not specified, the system default configuration will be used
// instead
func WithSignaturePolicy(path string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.SignaturePolicyPath = path

		return nil
	}
}

// WithInMemoryState specifies that the runtime will be backed by an in-memory
// state only, and state will not persist after the runtime is shut down
func WithInMemoryState() RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.InMemoryState = true

		return nil
	}
}

// WithOCIRuntime specifies an OCI runtime to use for running containers
func WithOCIRuntime(runtimePath string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.RuntimePath = runtimePath

		return nil
	}
}

// WithConmonPath specifies the path to the conmon binary which manages the
// runtime
func WithConmonPath(path string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.ConmonPath = path

		return nil
	}
}

// WithConmonEnv specifies the environment variable list for the conmon process
func WithConmonEnv(environment []string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.ConmonEnvVars = make([]string, len(environment))
		copy(rt.config.ConmonEnvVars, environment)

		return nil
	}
}

// WithCgroupManager specifies the manager implementation name which is used to
// handle cgroups for containers
// Current valid values are "cgroupfs" and "systemd"
func WithCgroupManager(manager string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.CgroupManager = manager

		return nil
	}
}

// WithStaticDir sets the directory that static runtime files which persist
// across reboots will be stored
func WithStaticDir(dir string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.StaticDir = dir

		return nil
	}
}

// WithTmpDir sets the directory that temporary runtime files which are not
// expected to survive across reboots will be stored
// This should be located on a tmpfs mount (/tmp or /var/run for example)
func WithTmpDir(dir string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.TmpDir = dir

		return nil
	}
}

// WithSELinux enables SELinux on the container server
func WithSELinux() RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.SelinuxEnabled = true

		return nil
	}
}

// WithPidsLimit specifies the maximum number of processes each container is
// restricted to
func WithPidsLimit(limit int64) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.PidsLimit = limit

		return nil
	}
}

// WithMaxLogSize sets the maximum size of container logs
// Positive sizes are limits in bytes, -1 is unlimited
func WithMaxLogSize(limit int64) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.MaxLogSize = limit

		return nil
	}
}

// WithNoPivotRoot sets the runtime to use MS_MOVE instead of PIVOT_ROOT when
// starting containers
func WithNoPivotRoot(noPivot bool) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.NoPivotRoot = true

		return nil
	}
}

// Container Creation Options

// WithShmDir sets the directory that should be mounted on /dev/shm
func WithShmDir(dir string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		ctr.config.ShmDir = dir
		return nil
	}
}

// WithSELinuxLabels sets the mount label for SELinux
func WithSELinuxLabels(processLabel, mountLabel string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		ctr.config.ProcessLabel = processLabel
		ctr.config.MountLabel = mountLabel
		return nil
	}
}

// WithRootFSFromImage sets up a fresh root filesystem using the given image
// If useImageConfig is specified, image volumes, environment variables, and
// other configuration from the image will be added to the config
// TODO: Replace image name and ID with a libpod.Image struct when that is finished
func WithRootFSFromImage(imageID string, imageName string, useImageConfig bool) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		if ctr.config.RootfsImageID != "" || ctr.config.RootfsImageName != "" {
			return errors.Wrapf(ErrInvalidArg, "container already configured with root filesystem")
		}

		ctr.config.RootfsImageID = imageID
		ctr.config.RootfsImageName = imageName
		ctr.config.UseImageConfig = useImageConfig

		return nil
	}
}

// WithStdin keeps stdin on the container open to allow interaction
func WithStdin() CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		ctr.config.Stdin = true

		return nil
	}
}

// WithSharedNamespaces sets a container to share namespaces with another
// container. If the from container belongs to a pod, the new container will
// be added to the pod.
// By default no namespaces are shared. To share a namespace, add the Namespace
// string constant to the map as a key
func WithSharedNamespaces(from *Container, namespaces map[string]string) CtrCreateOption {
	return ctrNotImplemented
}

// WithPod adds the container to a pod
func (r *Runtime) WithPod(pod *Pod) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		if pod == nil {
			return ErrInvalidArg
		}

		ctr.config.Pod = pod.ID()
		ctr.pod = pod

		return nil
	}
}

// WithLabels adds labels to the container
func WithLabels(labels map[string]string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		ctr.config.Labels = make(map[string]string)
		for key, value := range labels {
			ctr.config.Labels[key] = value
		}

		return nil
	}
}

// WithName sets the container's name
func WithName(name string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		ctr.config.Name = name

		return nil
	}
}

// WithStopSignal sets the signal that will be sent to stop the container
func WithStopSignal(signal uint) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		if signal == 0 {
			return errors.Wrapf(ErrInvalidArg, "stop signal cannot be 0")
		} else if signal > 64 {
			return errors.Wrapf(ErrInvalidArg, "stop signal cannot be greater than 64 (SIGRTMAX)")
		}

		ctr.config.StopSignal = signal

		return nil
	}
}

// Pod Creation Options

// WithPodName sets the name of the pod
func WithPodName(name string) PodCreateOption {
	return func(pod *Pod) error {
		if pod.valid {
			return ErrPodFinalized
		}

		pod.name = name

		return nil
	}
}

// WithPodLabels sets the labels of a pod
func WithPodLabels(labels map[string]string) PodCreateOption {
	return func(pod *Pod) error {
		if pod.valid {
			return ErrPodFinalized
		}

		pod.labels = make(map[string]string)
		for key, value := range labels {
			pod.labels[key] = value
		}

		return nil
	}
}
