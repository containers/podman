####> This option file is used in:
####>   podman exec, run
####> If you edit this file, make sure your changes
####> are applicable to all of those files.
#### **--preserve-fds**=*N*

Pass down to the process N additional file descriptors (in addition to 0, 1, 2).
The total FDs will be 3+N.
(This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)
