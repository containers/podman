#### **--device**=*host-device[:container-device][:permissions]*

Add a host device to the <<container|pod>>. Optional *permissions* parameter
can be used to specify device permissions by combining
**r** for read, **w** for write, and **m** for **mknod**(2).

Example: **--device=/dev/sdc:/dev/xvdc:rwm**.

Note: if _host-device_ is a symbolic link then it will be resolved first.
The <<container|pod>> will only store the major and minor numbers of the host device.

Note: if the user only has access rights via a group, accessing the device
from inside a rootless container will fail. Use the `--group-add keep-groups`
flag to pass the user's supplementary group access into the container.

Podman may load kernel modules required for using the specified
device. The devices that Podman will load modules for when necessary are:
/dev/fuse.
