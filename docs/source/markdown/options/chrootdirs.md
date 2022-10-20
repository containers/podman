####> This option file is used in:
####>   podman create, run
####> If you edit this file, make sure your changes
####> are applicable to all of those.
#### **--chrootdirs**=*path*

Path to a directory inside the container that should be treated as a `chroot` directory.
Any Podman managed file (e.g., /etc/resolv.conf, /etc/hosts, etc/hostname) that is mounted into the root directory will be mounted into that location as well.
Multiple directories should be separated with a comma.
