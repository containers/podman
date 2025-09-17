####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--cgroups**=*how*

Determines whether the container creates CGroups.

Default is **enabled**.

The **enabled** option creates a new cgroup under the cgroup-parent.
The **disabled** option forces the container to not create CGroups, and thus conflicts with CGroup options (**--cgroupns** and **--cgroup-parent**).
The **no-conmon** option disables a new CGroup only for the **conmon** process.
The **split** option splits the current CGroup in two sub-cgroups: one for conmon and one for the container payload. It is not possible to set **--cgroup-parent** with **split**.
