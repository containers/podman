#### **--pod**=*name*

Run container in an existing pod. If you want Podman to make the pod for you, prefix the pod name with **new:**.
To make a pod with more granular options, use the **podman pod create** command before creating a container.
If a container is run with a pod, and the pod has an infra-container, the infra-container will be started before the container is.
