#### **--device**=*host-device[:container-device][:permissions]*

Add a host device to the <POD-OR-CONTAINER>. Optional *permissions* parameter
can be used to specify device permissions by combining
**r** for read, **w** for write, and **m** for **mknod**(2).

Example: **--device=/dev/sdc:/dev/xvdc:rwm**.

Note: if _host_device_ is a symbolic link then it will be resolved first.
The <POD-OR-CONTAINER> will only store the major and minor numbers of the host device.

Note: the pod implements devices by storing the initial configuration passed by the user and recreating the device on each container added to the pod.

Podman may load kernel modules required for using the specified
device. The devices that Podman will load modules for when necessary are:
/dev/fuse.
