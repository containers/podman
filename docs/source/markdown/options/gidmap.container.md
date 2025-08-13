####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--gidmap**=*[flags]container_gid:from_gid[:amount]*

Run the container in a new user namespace using the supplied GID mapping. This
option conflicts with the **--userns** option. It provides a way to map host
GIDs to container GIDs in the same way as __--uidmap__ maps host UIDs to
container UIDs. For details see __--uidmap__.

Note: **--gidmap** cannot be called in conjunction with the **--pod** option,
because a gidmap cannot be set on the container level when in a pod.
