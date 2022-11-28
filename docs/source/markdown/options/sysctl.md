####> This option file is used in:
####>   podman create, pod clone, pod create, run
####> If you edit this file, make sure your changes
####> are applicable to all of those files.
#### **--sysctl**=*name=value*

Configure namespaced kernel parameters <<at runtime|for all containers in the pod>>.

For the IPC namespace, the following sysctls are allowed:

- kernel.msgmax
- kernel.msgmnb
- kernel.msgmni
- kernel.sem
- kernel.shmall
- kernel.shmmax
- kernel.shmmni
- kernel.shm_rmid_forced
- Sysctls beginning with fs.mqueue.\*

Note: <<if you use the **--ipc=host** option|if the ipc namespace is not shared within the pod>>, the above sysctls are not allowed.

For the network namespace, only sysctls beginning with net.\* are allowed.

Note: <<if you use the **--network=host** option|if the network namespace is not shared within the pod>>, the above sysctls are not allowed.
