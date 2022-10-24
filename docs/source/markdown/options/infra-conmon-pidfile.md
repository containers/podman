####> This option file is used in:
####>   podman pod clone, pod create
####> If you edit this file, make sure your changes
####> are applicable to all of those.
#### **--infra-conmon-pidfile**=*file*

Write the pid of the infra container's **conmon** process to a file. As **conmon** runs in a separate process than Podman, this is necessary when using systemd to manage Podman containers and pods.
