####> This option file is used in:
####>   podman create, run
####> If you edit this file, make sure your changes
####> are applicable to all of those files.
#### **--read-only**

Mount the container's root filesystem as read-only.

By default, containers have their root filesystem writable, allowing processes
to write files anywhere. By specifying the **--read-only** flag, containers
will have root filesystems mounted as read-only, prohibiting any writes.
