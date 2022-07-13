#### **--cgroups**=*how*

Determines whether the container will create CGroups.

Default is **enabled**.

The **enabled** option will create a new cgroup under the cgroup-parent.
The **disabled** option will force the container to not create CGroups, and thus conflicts with CGroup options (**--cgroupns** and **--cgroup-parent**).
The **no-conmon** option disables a new CGroup only for the **conmon** process.
The **split** option splits the current CGroup in two sub-cgroups: one for conmon and one for the container payload. It is not possible to set **--cgroup-parent** with **split**.
