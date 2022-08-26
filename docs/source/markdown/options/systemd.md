#### **--systemd**=*true* | *false* | *always*

Run container in systemd mode. The default is **true**.

The value *always* enforces the systemd mode is enforced without
looking at the executable name. Otherwise, if set to true and the
command you are running inside the container is **systemd**, **/usr/sbin/init**,
**/sbin/init** or **/usr/local/sbin/init**.

Running the container in systemd mode causes the following changes:

* Podman mounts tmpfs file systems on the following directories
  * _/run_
  * _/run/lock_
  * _/tmp_
  * _/sys/fs/cgroup/systemd_
  * _/var/lib/journal_
* Podman sets the default stop signal to **SIGRTMIN+3**.
* Podman sets **container_uuid** environment variable in the container to the
first 32 characters of the container id.

This allows systemd to run in a confined container without any modifications.

Note that on **SELinux** systems, systemd attempts to write to the cgroup
file system. Containers writing to the cgroup file system are denied by default.
The **container_manage_cgroup** boolean must be enabled for this to be allowed on an SELinux separated system.
```
setsebool -P container_manage_cgroup true
```
