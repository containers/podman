####> This option file is used in:
####>   podman build, create, pod clone, pod create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--device**=*host-device[:container-device][:permissions]*

Add a host device to the <<container|pod>>. Optional *permissions* parameter
can be used to specify device permissions by combining
**r** for read, **w** for write, and **m** for **mknod**(2).

Example: **--device=/dev/sdc:/dev/xvdc:rwm**.

Note: if *host-device* is a symbolic link then it will be resolved first.
The <<container|pod>> will only store the major and minor numbers of the host device.

Podman may load kernel modules required for using the specified
device. The devices that Podman will load modules for when necessary are:
/dev/fuse.

In rootless mode, the new device is bind mounted in the container from the host
rather than Podman creating it within the container space. Because the bind
mount retains its SELinux label on SELinux systems, the container can get
permission denied when accessing the mounted device. Modify SELinux settings to
allow containers to use all device labels via the following command:

$ sudo setsebool -P  container_use_devices=true
