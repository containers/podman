#### **--uidmap**=*container_uid:from_uid:amount*

Run all containers in the pod in a new user namespace using the supplied mapping. This
option conflicts with the **--userns** and **--subuidname** options. This
option provides a way to map host UIDs to container UIDs. It can be passed
several times to map different ranges.
