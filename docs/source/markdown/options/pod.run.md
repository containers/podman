####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--pod**=*name*

Run container in an existing pod. Podman will make the pod automatically if the pod name is prefixed with **new:**.
To make a pod with more granular options, use the **podman pod create** command before creating a container.
If a container is run with a pod, and the pod has an infra-container, the infra-container will be started before the container is.
