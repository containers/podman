####> This option file is used in:
####>   podman pod clone, pod create
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--uidmap**=*container_uid:from_uid[:amount]*

Run all containers in the pod in a new user namespace using the supplied UID
mapping. This option conflicts with the **--userns** option. It provides a way
to map host UIDs to container UIDs. It can be passed several times to map
different ranges.
