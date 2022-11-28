####> This option file is used in:
####>   podman container clone, create, run, update
####> If you edit this file, make sure your changes
####> are applicable to all of those files.
#### **--cpu-rt-period**=*microseconds*

Limit the CPU real-time period in microseconds.

Limit the container's Real Time CPU usage. This option tells the kernel to restrict the container's Real Time CPU usage to the period specified.

This option is only supported on cgroups V1 rootful systems.
