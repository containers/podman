####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--mount**=*type=TYPE,TYPE-SPECIFIC-OPTION[,...]*

Attach a filesystem mount to the container

Current supported mount TYPEs are **bind**, **devpts**, **glob**, **image**, **ramfs**, **tmpfs** and **volume**.

Options common to all mount types:

- *src*, *source*: mount source spec for **bind**, **glob**, and **volume**.
  Mandatory for **bind** and **glob**.

- *dst*, *destination*, *target*: mount destination spec.

When source globs are specified without the destination directory,
the files and directories are mounted with their complete path
within the container. When the destination is specified, the
files and directories matching the glob on the base file name
on the destination directory are mounted. The option
`type=glob,src=/foo*,destination=/tmp/bar` tells container engines
to mount host files matching /foo* to the /tmp/bar/
directory in the container.

Options specific to type=**volume**:

- *ro*, *readonly*: *true* or *false* (default if unspecified: *false*).

- *U*, *chown*: *true* or *false* (default if unspecified: *false*). Recursively change the owner and group of the source volume based on the UID and GID of the container.

- *idmap*: If specified, create an idmapped mount to the target user namespace in the container.
  The idmap option is only supported by Podman in rootful mode. The Linux kernel does not allow the use of idmaped file systems for unprivileged users.
  The idmap option supports a custom mapping that can be different than the user namespace used by the container.
  The mapping can be specified after the idmap option like: `idmap=uids=0-1-10#10-11-10;gids=0-100-10`.  For each triplet, the first value is the
  start of the backing file system IDs that are mapped to the second value on the host.  The length of this mapping is given in the third value.
  Multiple ranges are separated with #.  If the specified mapping is prepended with a '@' then the mapping is considered relative to the container
  user namespace. The host ID for the mapping is changed to account for the relative position of the container user in the container user namespace.

Options specific to type=**image**:

- *rw*, *readwrite*: *true* or *false* (default if unspecified: *false*).

- *subpath*: Mount only a specific path within the image, instead of the whole image.

Options specific to **bind** and **glob**:

- *ro*, *readonly*: *true* or *false* (default if unspecified: *false*).

- *bind-propagation*: *shared*, *slave*, *private*, *unbindable*, *rshared*, *rslave*, *runbindable*, or **rprivate** (default).<sup>[[1]](#Footnote1)</sup> See also mount(2).

- *bind-nonrecursive*: do not set up a recursive bind mount. By default it is recursive.

- *relabel*: *shared*, *private*.

- *idmap*: *true* or *false* (default if unspecified: *false*).  If true, create an idmapped mount to the target user namespace in the container. The idmap option is only supported by Podman in rootful mode.

- *U*, *chown*: *true* or *false* (default if unspecified: *false*). Recursively change the owner and group of the source volume based on the UID and GID of the container.

- *no-dereference*: do not dereference symlinks but copy the link source into the mount destination.

Options specific to type=**tmpfs** and **ramfs**:

- *ro*, *readonly*: *true* or *false* (default if unspecified: *false*).

- *tmpfs-size*: Size of the tmpfs/ramfs mount, in bytes. Unlimited by default in Linux.

- *tmpfs-mode*: Octal file mode of the tmpfs/ramfs (e.g. 700 or 0700.).

- *tmpcopyup*: Enable copyup from the image directory at the same location to the tmpfs/ramfs. Used by default.

- *notmpcopyup*: Disable copying files from the image to the tmpfs/ramfs.

- *U*, *chown*: *true* or *false* (default if unspecified: *false*). Recursively change the owner and group of the source volume based on the UID and GID of the container.

Options specific to type=**devpts**:

- *uid*: numeric UID of the file owner (default: 0).

- *gid*: numeric GID of the file owner (default: 0).

- *mode*: octal permission mask for the file (default: 600).

- *max*: maximum number of PTYs (default: 1048576).

Examples:

- `type=bind,source=/path/on/host,destination=/path/in/container`

- `type=bind,src=/path/on/host,dst=/path/in/container,relabel=shared`

- `type=bind,src=/path/on/host,dst=/path/in/container,relabel=shared,U=true`

- `type=devpts,destination=/dev/pts`

- `type=glob,src=/usr/lib/libfoo*,destination=/usr/lib,ro=true`

- `type=image,source=fedora,destination=/fedora-image,rw=true`

- `type=ramfs,tmpfs-size=512M,destination=/path/in/container`

- `type=tmpfs,tmpfs-size=512M,destination=/path/in/container`

- `type=tmpfs,destination=/path/in/container,noswap`

- `type=volume,source=vol1,destination=/path/in/container,ro=true`
