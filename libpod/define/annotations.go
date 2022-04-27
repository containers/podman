package define

const (
	// InspectAnnotationCIDFile is used by Inspect to determine if a
	// container ID file was created for the container.
	// If an annotation with this key is found in the OCI spec, it will be
	// used in the output of Inspect().
	InspectAnnotationCIDFile = "io.podman.annotations.cid-file"
	// InspectAnnotationAutoremove is used by Inspect to determine if a
	// container will be automatically removed on exit.
	// If an annotation with this key is found in the OCI spec and is one of
	// the two supported boolean values (InspectResponseTrue and
	// InspectResponseFalse) it will be used in the output of Inspect().
	InspectAnnotationAutoremove = "io.podman.annotations.autoremove"
	// InspectAnnotationVolumesFrom is used by Inspect to identify
	// containers whose volumes are are being used by this container.
	// It is expected to be a comma-separated list of container names and/or
	// IDs.
	// If an annotation with this key is found in the OCI spec, it will be
	// used in the output of Inspect().
	InspectAnnotationVolumesFrom = "io.podman.annotations.volumes-from"
	// InspectAnnotationPrivileged is used by Inspect to identify containers
	// which are privileged (IE, running with elevated privileges).
	// It is expected to be a boolean, populated by one of
	// InspectResponseTrue or InspectResponseFalse.
	// If an annotation with this key is found in the OCI spec, it will be
	// used in the output of Inspect().
	InspectAnnotationPrivileged = "io.podman.annotations.privileged"
	// InspectAnnotationPublishAll is used by Inspect to identify containers
	// which have all the ports from their image published.
	// It is expected to be a boolean, populated by one of
	// InspectResponseTrue or InspectResponseFalse.
	// If an annotation with this key is found in the OCI spec, it will be
	// used in the output of Inspect().
	InspectAnnotationPublishAll = "io.podman.annotations.publish-all"
	// InspectAnnotationInit is used by Inspect to identify containers that
	// mount an init binary in.
	// It is expected to be a boolean, populated by one of
	// InspectResponseTrue or InspectResponseFalse.
	// If an annotation with this key is found in the OCI spec, it will be
	// used in the output of Inspect().
	InspectAnnotationInit = "io.podman.annotations.init"
	// InspectAnnotationLabel is used by Inspect to identify containers with
	// special SELinux-related settings. It is used to populate the output
	// of the SecurityOpt setting.
	// If an annotation with this key is found in the OCI spec, it will be
	// used in the output of Inspect().
	InspectAnnotationLabel = "io.podman.annotations.label"
	// InspectAnnotationSeccomp is used by Inspect to identify containers
	// with special Seccomp-related settings. It is used to populate the
	// output of the SecurityOpt setting in Inspect.
	// If an annotation with this key is found in the OCI spec, it will be
	// used in the output of Inspect().
	InspectAnnotationSeccomp = "io.podman.annotations.seccomp"
	// InspectAnnotationApparmor is used by Inspect to identify containers
	// with special Apparmor-related settings. It is used to populate the
	// output of the SecurityOpt setting.
	// If an annotation with this key is found in the OCI spec, it will be
	// used in the output of Inspect().
	InspectAnnotationApparmor = "io.podman.annotations.apparmor"

	// InspectResponseTrue is a boolean True response for an inspect
	// annotation.
	InspectResponseTrue = "TRUE"
	// InspectResponseFalse is a boolean False response for an inspect
	// annotation.
	InspectResponseFalse = "FALSE"

	// CheckpointAnnotationName is used by Container Checkpoint when creating a
	// checkpoint image to specify the original human-readable name for the
	// container.
	CheckpointAnnotationName = "io.podman.annotations.checkpoint.name"

	// CheckpointAnnotationRawImageName is used by Container Checkpoint when
	// creating a checkpoint image to specify the original unprocessed name of
	// the image used to create the container (as specified by the user).
	CheckpointAnnotationRawImageName = "io.podman.annotations.checkpoint.rawImageName"

	// CheckpointAnnotationRootfsImageID is used by Container Checkpoint when
	// creating a checkpoint image to specify the original ID of the image used
	// to create the container.
	CheckpointAnnotationRootfsImageID = "io.podman.annotations.checkpoint.rootfsImageID"

	// CheckpointAnnotationRootfsImageName is used by Container Checkpoint when
	// creating a checkpoint image to specify the original image name used to
	// create the container.
	CheckpointAnnotationRootfsImageName = "io.podman.annotations.checkpoint.rootfsImageName"

	// CheckpointAnnotationPodmanVersion is used by Container Checkpoint when
	// creating a checkpoint image to specify the version of Podman used on the
	// host where the checkpoint was created.
	CheckpointAnnotationPodmanVersion = "io.podman.annotations.checkpoint.podman.version"

	// CheckpointAnnotationCriuVersion is used by Container Checkpoint when
	// creating a checkpoint image to specify the version of CRIU used on the
	// host where the checkpoint was created.
	CheckpointAnnotationCriuVersion = "io.podman.annotations.checkpoint.criu.version"

	// CheckpointAnnotationRuntimeName is used by Container Checkpoint when
	// creating a checkpoint image to specify the runtime used on the host where
	// the checkpoint was created.
	CheckpointAnnotationRuntimeName = "io.podman.annotations.checkpoint.runtime.name"

	// CheckpointAnnotationRuntimeVersion is used by Container Checkpoint when
	// creating a checkpoint image to specify the version of runtime used on the
	// host where the checkpoint was created.
	CheckpointAnnotationRuntimeVersion = "io.podman.annotations.checkpoint.runtime.version"

	// CheckpointAnnotationConmonVersion is used by Container Checkpoint when
	// creating a checkpoint image to specify the version of conmon used on
	// the host where the checkpoint was created.
	CheckpointAnnotationConmonVersion = "io.podman.annotations.checkpoint.conmon.version"

	// CheckpointAnnotationHostArch is used by Container Checkpoint when
	// creating a checkpoint image to specify the CPU architecture of the host
	// on which the checkpoint was created.
	CheckpointAnnotationHostArch = "io.podman.annotations.checkpoint.host.arch"

	// CheckpointAnnotationHostKernel is used by Container Checkpoint when
	// creating a checkpoint image to specify the kernel version used by the
	// host where the checkpoint was created.
	CheckpointAnnotationHostKernel = "io.podman.annotations.checkpoint.host.kernel"

	// CheckpointAnnotationCgroupVersion is used by Container Checkpoint when
	// creating a checkpoint image to specify the cgroup version used by the
	// host where the checkpoint was created.
	CheckpointAnnotationCgroupVersion = "io.podman.annotations.checkpoint.cgroups.version"

	// CheckpointAnnotationDistributionVersion is used by Container Checkpoint
	// when creating a checkpoint image to specify the version of host
	// distribution on which the checkpoint was created.
	CheckpointAnnotationDistributionVersion = "io.podman.annotations.checkpoint.distribution.version"

	// CheckpointAnnotationDistributionName is used by Container Checkpoint when
	// creating a checkpoint image to specify the name of host distribution on
	// which the checkpoint was created.
	CheckpointAnnotationDistributionName = "io.podman.annotations.checkpoint.distribution.name"
	// MaxKubeAnnotation is the max length of annotations allowed by Kubernetes.
	MaxKubeAnnotation = 63
)

// IsReservedAnnotation returns true if the specified value corresponds to an
// already reserved annotation that Podman sets during container creation.
func IsReservedAnnotation(value string) bool {
	switch value {
	case InspectAnnotationCIDFile, InspectAnnotationAutoremove, InspectAnnotationVolumesFrom, InspectAnnotationPrivileged, InspectAnnotationPublishAll, InspectAnnotationInit, InspectAnnotationLabel, InspectAnnotationSeccomp, InspectAnnotationApparmor, InspectResponseTrue, InspectResponseFalse:
		return true

	default:
		return false
	}
}
