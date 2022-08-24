#### **--gidmap**=*pod_gid:host_gid:amount*

GID map for the user namespace. Using this flag will run all containers in the pod with user namespace enabled.
It conflicts with the **--userns** and **--subgidname** flags.
