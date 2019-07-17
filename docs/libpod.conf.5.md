% libpod.conf(5)

## NAME
libpod.conf - libpod configuration file

## DESCRIPTION
The libpod.conf file is the default configuration file for all tools using
libpod to manage containers.

## OPTIONS

**image_default_transport**=""
  Default transport method for pulling and pushing images

**runtime**=""
  Default OCI runtime to use if nothing is specified in **runtimes**

**runtimes**
  For each OCI runtime, specify a list of paths to look for.  The first one found is used.

**conmon_path**=""
  Paths to search for the Conmon container manager binary

**conmon_env_vars**=""
  Environment variables to pass into Conmon

**cgroup_manager**=""
  Specify the CGroup Manager to use; valid values are "systemd" and "cgroupfs"

**lock_type**=""
  Specify the locking mechanism to use; valid values are "shm" and "file".  Change the default only if you are sure of what you are doing, in general "file" is useful only on platforms where cgo is not available for using the faster "shm" lock type.  You may need to run "podman system renumber" after you change the lock type.

**init_path**=""
  Path to the container-init binary, which forwards signals and reaps processes within containers.  Note that the container-init binary will only be used when the `--init` for podman-create and podman-run is set.

**hooks_dir**=["*path*", ...]

  Each `*.json` file in the path configures a hook for Podman containers.  For more details on the syntax of the JSON files and the semantics of hook injection, see `oci-hooks(5)`.  Podman and libpod currently support both the 1.0.0 and 0.1.0 hook schemas, although the 0.1.0 schema is deprecated.

  Paths listed later in the array have higher precedence (`oci-hooks(5)` discusses directory precedence).

  For the annotation conditions, libpod uses any annotations set in the generated OCI configuration.

  For the bind-mount conditions, only mounts explicitly requested by the caller via `--volume` are considered.  Bind mounts that libpod inserts by default (e.g. `/dev/shm`) are not considered.

  Podman and libpod currently support an additional `precreate` state which is called before the runtime's `create` operation.  Unlike the other stages, which receive the container state on their standard input, `precreate` hooks receive the proposed runtime configuration on their standard input.  They may alter that configuration as they see fit, and write the altered form to their standard output.

  **WARNING**: the `precreate` hook lets you do powerful things, such as adding additional mounts to the runtime configuration.  That power also makes it easy to break things.  Before reporting libpod errors, try running your container with `precreate` hooks disabled to see if the problem is due to one of your hooks.

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

**infra_image** = ""
  Infra (pause) container image name for pod infra containers.  When running a pod, we
  start a `pause` process in a container to hold open the namespaces associated with the
  pod.  This container and process, basically sleep/pause for the lifetime of the pod.

**infra_command**=""
  Command to run the infra container

**namespace**=""
  Default libpod namespace. If libpod is joined to a namespace, it will see only containers and pods
  that were created in the same namespace, and will create new containers and pods in that namespace.
  The default namespace is "", which corresponds to no namespace. When no namespace is set, all
  containers and pods are visible.

**label**="true|false"
  Indicates whether the containers should use label separation.

**num_locks**=""
  Number of locks available for containers and pods. Each created container or pod consumes one lock.
  The default number available is 2048.
  If this is changed, a lock renumbering must be performed, using the `podman system renumber` command.

**volume_path**=""
  Directory where named volumes will be created in using the default volume driver.
  By default this will be configured relative to where containers/storage stores containers.

**network_cmd_path**=""
  Path to the command binary to use for setting up a network.  It is currently only used for setting up
  a slirp4netns network.  If "" is used then the binary is looked up using the $PATH environment variable.

**events_logger**=""
  Default method to use when logging events. Valid values are "file", "journald", and "null".

**detach_keys**=""
  Keys sequence used for detaching a container

## FILES
  `/usr/share/containers/libpod.conf`, default libpod configuration path

  `/etc/containers/libpod.conf`, override libpod configuration path

## HISTORY
Apr 2018, Originally compiled by Nathan Williams <nath.e.will@gmail.com>
