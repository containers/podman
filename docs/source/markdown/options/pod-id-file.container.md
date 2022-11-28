####> This option file is used in:
####>   podman create, run
####> If you edit this file, make sure your changes
####> are applicable to all of those files.
#### **--pod-id-file**=*file*

Run container in an existing pod and read the pod's ID from the specified *file*.
If a container is run within a pod, and the pod has an infra-container, the infra-container will be started before the container is.
