####> This option file is used in:
####>   podman create, pull, run
####> If you edit this file, make sure your changes
####> are applicable to all of those files.
#### **--platform**=*OS/ARCH*

Specify the platform for selecting the image.  (Conflicts with --arch and --os)
The `--platform` option can be used to override the current architecture and operating system.
Unless overridden, subsequent lookups of the same image in the local storage will match this platform, regardless of the host.
