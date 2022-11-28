####> This option file is used in:
####>   podman create, run
####> If you edit this file, make sure your changes
####> are applicable to all of those files.
#### **--passwd-entry**=*ENTRY*

Customize the entry that is written to the `/etc/passwd` file within the container when `--passwd` is used.

The variables $USERNAME, $UID, $GID, $NAME, $HOME are automatically replaced with their value at runtime.
