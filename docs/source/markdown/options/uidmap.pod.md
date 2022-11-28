####> This option file is used in:
####>   podman pod clone, pod create
####> If you edit this file, make sure your changes
####> are applicable to all of those files.
#### **--uidmap**=*container_uid:from_uid:amount*

Run all containers in the pod in a new user namespace using the supplied mapping. This
option conflicts with the **--userns** and **--subuidname** options. This
option provides a way to map host UIDs to container UIDs. It can be passed
several times to map different ranges.
