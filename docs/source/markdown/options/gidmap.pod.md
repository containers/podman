####> This option file is used in:
####>   podman pod clone, pod create
####> If you edit this file, make sure your changes
####> are applicable to all of those files.
#### **--gidmap**=*pod_gid:host_gid:amount*

GID map for the user namespace. Using this flag will run all containers in the pod with user namespace enabled.
It conflicts with the **--userns** and **--subgidname** flags.
