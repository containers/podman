#### **--group-add**=*group* | *keep-groups*

Assign additional groups to the primary user running within the container process.

- `keep-groups` is a special flag that tells Podman to keep the supplementary group access.

Allows container to use the user's supplementary group access. If file systems or
devices are only accessible by the rootless user's group, this flag tells the OCI
runtime to pass the group access into the container. Currently only available
with the `crun` OCI runtime. Note: `keep-groups` is exclusive, you cannot add any other groups
with this flag. (Not available for remote commands, including Mac and Windows (excluding WSL2) machines)
