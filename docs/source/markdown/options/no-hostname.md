####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--no-hostname**

Do not create the _/etc/hostname_ file in the container for RUN instructions.

By default, Buildah manages the _/etc/hostname_ file, adding the container's own hostname.  When the **--no-hostname** option is set, the image's _/etc/hostname_ will be preserved unmodified if it exists.
