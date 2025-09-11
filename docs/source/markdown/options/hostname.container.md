####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--hostname**, **-h**=*name*

Set the container's hostname inside the container.

This option can only be used with a private UTS namespace `--uts=private`
(default). If `--pod` is given and the pod shares the same UTS namespace
(default), the pod's hostname is used. The given hostname is also added to the
`/etc/hosts` file using the container's primary IP address (also see the
**--add-host** option).
