####> This option file is used in:
####>   podman create, pull, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--os**=*OS*

Override the OS, defaults to hosts, of the image to be pulled. For example, `windows`.
Unless overridden, subsequent lookups of the same image in the local storage will match this OS, regardless of the host.
