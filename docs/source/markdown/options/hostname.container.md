#### **--hostname**, **-h**=*name*

Container host name

Sets the container host name that is available inside the container. Can only be used with a private UTS namespace `--uts=private` (default). If `--pod` is specified and the pod shares the UTS namespace (default) the pod's hostname will be used.
