####> This option file is used in:
####>   podman create, run
####> If you edit this file, make sure your changes
####> are applicable to all of those.
#### **--read-write-tmpfs**

If container is running in --read-only mode, when true Podman mounts a
read-write tmpfs on _/run_, _/tmp_, and _/var/tmp_. When false, Podman sets
the entire container such that no tmpfs can be written to unless specified on
the command line.  Podman mounts _/dev_, _/dev/mqueue_, _/dev/pts_, _/dev/shm_
as read only. The default is *true*
