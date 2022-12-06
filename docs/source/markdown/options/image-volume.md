####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--image-volume**=**bind** | *tmpfs* | *ignore*

Tells Podman how to handle the builtin image volumes. Default is **bind**.

- **bind**: An anonymous named volume will be created and mounted into the container.
- **tmpfs**: The volume is mounted onto the container as a tmpfs, which allows the users to create
content that disappears when the container is stopped.
- **ignore**: All volumes are just ignored and no action is taken.
