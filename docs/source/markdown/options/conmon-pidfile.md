#### **--conmon-pidfile**=*file*

Write the pid of the **conmon** process to a file. As **conmon** runs in a separate process than Podman, this is necessary when using systemd to restart Podman containers.
(This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)
