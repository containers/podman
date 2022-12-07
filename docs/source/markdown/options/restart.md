####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--restart**=*policy*

Restart policy to follow when containers exit.
Restart policy will not take effect if a container is stopped via the **podman kill** or **podman stop** commands.

Valid _policy_ values are:

- `no`                       : Do not restart containers on exit
- `on-failure[:max_retries]` : Restart containers when they exit with a non-zero exit code, retrying indefinitely or until the optional *max_retries* count is hit
- `always`                   : Restart containers when they exit, regardless of status, retrying indefinitely
- `unless-stopped`           : Identical to **always**

Podman provides a systemd unit file, podman-restart.service, which restarts containers after a system reboot.

If container will run as a system service, generate a systemd unit file to manage it. See **podman generate systemd**.
