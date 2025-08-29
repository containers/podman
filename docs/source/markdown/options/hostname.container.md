####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
{% if is_quadlet %}
### `HostName=name`
{% else %}
#### **--hostname**, **-h**=*name*
{% endif %}

Set the container's hostname inside the container.

This option can only be used with a private UTS namespace `--uts=private`
(default). If {{{ '`Pod=`' if is_quadlet else '`--pod`' }}} is given and the pod shares the same UTS namespace
(default), the pod's hostname is used. The given hostname is also added to the
`/etc/hosts` file using the container's primary IP address (also see the
{{{ '**AddHost=**' if is_quadlet else '**--add-host**' }}} option).
