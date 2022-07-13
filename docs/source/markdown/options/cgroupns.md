#### **--cgroupns**=*mode*

Set the cgroup namespace mode for the container.

- **host**: use the host's cgroup namespace inside the container.
- **container:<NAME|ID>**: join the namespace of the specified container.
- **private**: create a new cgroup namespace.
- **ns:<PATH>**: join the namespace at the specified path.

If the host uses cgroups v1, the default is set to **host**. On cgroups v2, the default is **private**.
