####> This option file is used in:
####>   podman build, container clone, create, pod clone, pod create, run, update
####> If you edit this file, make sure your changes
####> are applicable to all of those files.
#### **--memory**, **-m**=*number[unit]*

Memory limit. A _unit_ can be **b** (bytes), **k** (kibibytes), **m** (mebibytes), or **g** (gibibytes).

Allows the memory available to a container to be constrained. If the host
supports swap memory, then the **-m** memory setting can be larger than physical
RAM. If a limit of 0 is specified (not using **-m**), the container's memory is
not limited. The actual limit may be rounded up to a multiple of the operating
system's page size (the value would be very large, that's millions of trillions).

This option is not supported on cgroups V1 rootless systems.
