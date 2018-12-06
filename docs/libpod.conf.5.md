% libpod.conf(5)

## NAME
libpod.conf - libpod configuration file

## DESCRIPTION
The libpod.conf file is the default configuration file for all tools using
libpod to manage containers.

## OPTIONS

**image_default_transport**=""
  Default transport method for pulling and pushing images

**runtime_path**=""
  Paths to search for a valid OCI runtime binary

**conmon_path**=""
  Paths to search for the Conmon container manager binary

**conmon_env_vars**=""
  Environment variables to pass into Conmon

**cgroup_manager**=""
  Specify the CGroup Manager to use; valid values are "systemd" and "cgroupfs"

**hooks_dir**=["*path*", ...]

  Each `*.json` file in the path configures a hook for Podman containers.  For more details on the syntax of the JSON files and the semantics of hook injection, see `oci-hooks(5)`.  Podman and libpod currently support both the 1.0.0 and 0.1.0 hook schemas, although the 0.1.0 schema is deprecated.

  Paths listed later in the array higher precedence (`oci-hooks(5)` discusses directory precedence).

  For the annotation conditions, libpod uses any annotations set in the generated OCI configuration.

  For the bind-mount conditions, only mounts explicitly requested by the caller via `--volume` are considered.  Bind mounts that libpod inserts by default (e.g. `/dev/shm`) are not considered.

  If `hooks_dir` is unset for root callers, Podman and libpod will currently default to `/usr/share/containers/oci/hooks.d` and `/etc/containers/oci/hooks.d` in order of increasing precedence.  Using these defaults is deprecated, and callers should migrate to explicitly setting `hooks_dir`.

**static_dir**=""
  Directory for persistent libpod files (database, etc)
  By default this will be configured relative to where containers/storage
  stores containers

**tmp_dir**=""
  Directory for temporary files
  Must be a tmpfs (wiped after reboot)

**max_log_size**=""
  Maximum size of log files (in bytes)

**no_pivot_root**=""
  Whether to use chroot instead of pivot_root in the runtime

**cni_config_dir**=""
  Directory containing CNI plugin configuration files

**cni_plugin_dir**=""
  Directories where CNI plugin binaries may be located

**pause_image** = ""
  Pause container image name for pod pause containers.  When running a pod, we
  start a `pause` processes in a container to hold open the namespaces associated with the
  pod.  This container and process, basically sleep/pause for the lifetime of the pod.

**pause_command**=""
  Command to run the pause container

**namespace**=""
  Default libpod namespace. If libpod is joined to a namespace, it will see only containers and pods
  that were created in the same namespace, and will create new containers and pods in that namespace.
  The default namespace is "", which corresponds to no namespace. When no namespace is set, all
  containers and pods are visible.

**label**="true|false"
  Indicates whether the containers should use label separation.

## FILES
  `/usr/share/containers/libpod.conf`, default libpod configuration path

  `/etc/containers/libpod.conf`, override libpod configuration path

## HISTORY
Apr 2018, Originally compiled by Nathan Williams <nath.e.will@gmail.com>
