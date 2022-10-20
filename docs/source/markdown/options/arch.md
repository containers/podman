####> This option file is used in:
####>   podman create, pull, run
####> If you edit this file, make sure your changes
####> are applicable to all of those.
#### **--arch**=*ARCH*
Override the architecture, defaults to hosts, of the image to be pulled. For example, `arm`.
Unless overridden, subsequent lookups of the same image in the local storage will match this architecture, regardless of the host.
