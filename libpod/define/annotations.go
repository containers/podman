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
)
