#### **--restart**=*policy*

Restart policy to follow when containers exit.
Restart policy will not take effect if a container is stopped via the **podman kill** or **podman stop** commands.

Valid _policy_ values are:

- `no`                       : Do not restart containers on exit
- `on-failure[:max_retries]` : Restart containers when they exit with a non-zero exit code, retrying indefinitely or until the optional *max_retries* count is hit
- `always`                   : Restart containers when they exit, regardless of status, retrying indefinitely
- `unless-stopped`           : Identical to **always**

Please note that restart will not restart containers after a system reboot.
If this functionality is required in your environment, you can invoke Podman from a **systemd.unit**(5) file, or create an init script for whichever init system is in use.
To generate systemd unit files, please see **podman generate systemd**.
