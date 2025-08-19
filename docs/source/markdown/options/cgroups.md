####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
{% if is_quadlet %}
### `CgroupsMode=how`
{% else %}
#### **--cgroups**=*how*
{% endif %}

Determines whether the container creates CGroups.

{% if is_quadlet %}
By default, the cgroups mode of the container created by Quadlet is `split`,
which differs from the default (`enabled`) used by the Podman CLI.

If the container joins a pod (i.e. `Pod=` is specified), you may want to change this to
`no-conmon` or `enabled` so that pod level cgroup resource limits can take effect.
{% else %}
Default is **enabled**.
{% endif %}

The **enabled** option creates a new cgroup under the cgroup-parent.
The **disabled** option forces the container to not create CGroups, and thus conflicts with CGroup options (**--cgroupns** and **--cgroup-parent**).
The **no-conmon** option disables a new CGroup only for the **conmon** process.
The **split** option splits the current CGroup in two sub-cgroups: one for conmon and one for the container payload. It is not possible to set **--cgroup-parent** with **split**.
