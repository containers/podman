####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--gidmap**=*[flags]container_uid:from_uid[:amount]*

Run the container in a new user namespace using the supplied GID mapping. This
option conflicts with the **--userns** and **--subgidname** options. This
option provides a way to map host GIDs to container GIDs in the same way as
__--uidmap__ maps host UIDs to container UIDs. For details see __--uidmap__.

Note: the **--gidmap** option cannot be called in conjunction with the **--pod** option as a gidmap cannot be set on the container level when in a pod.
