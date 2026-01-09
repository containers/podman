####> This option file is used in:
####>   podman create, pod clone, pod create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--volume-mode**=*mode*

Control the behavior when a named volume specified with **--volume** or **--mount** does not exist.

Supported modes are:

- **create**: Automatically create the volume if it doesn't exist. This is the default behavior.
- **fail**: Return an error if the volume doesn't exist.

This option only affects named volumes. Bind mounts (host paths) and anonymous volumes are not affected by this option.

Example:

```
# Fail if volume doesn't exist
podman run --volume-mode=fail -v myvolume:/data alpine

# Explicitly use default behavior (auto-create)
podman run --volume-mode=create -v myvolume:/data alpine
```
