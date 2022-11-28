####> This option file is used in:
####>   podman pod clone, pod create
####> If you edit this file, make sure your changes
####> are applicable to all of those files.
#### **--uts**=*mode*

Set the UTS namespace mode for the pod. The following values are supported:

- **host**: use the host's UTS namespace inside the pod.
- **private**: create a new namespace for the pod (default).
- **ns:[path]**: run the pod in the given existing UTS namespace.
