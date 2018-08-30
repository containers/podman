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

## FILES
/etc/containers/libpod.conf, default libpod configuration path

## HISTORY
Apr 2018, Originally compiled by Nathan Williams <nath.e.will@gmail.com>
