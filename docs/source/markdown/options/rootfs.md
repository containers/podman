#### **--rootfs**

If specified, the first argument refers to an exploded container on the file system.

This is useful to run a container without requiring any image management, the rootfs
of the container is assumed to be managed externally.

  `Overlay Rootfs Mounts`

   The `:O` flag tells Podman to mount the directory from the rootfs path as
storage using the `overlay file system`. The container processes
can modify content within the mount point which is stored in the
container storage in a separate directory. In overlay terms, the source
directory will be the lower, and the container storage directory will be the
upper. Modifications to the mount point are destroyed when the container
finishes executing, similar to a tmpfs mount point being unmounted.

Note: On **SELinux** systems, the rootfs needs the correct label, which is by default
**unconfined_u:object_r:container_file_t:s0**.
