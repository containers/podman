# rootless-cni-infra

Infra container for CNI-in-slirp4netns.

## How it works

When a CNI network is specified for `podman run` in rootless mode, Podman launches the `rootless-cni-infra` container to execute CNI plugins inside slirp4netns.

The infra container is created per user, by executing an equivalent of:
`podman run -d --name rootless-cni-infra --pid=host --privileged -v $HOME/.config/cni/net.d:/etc/cni/net.d rootless-cni-infra`.
The infra container is automatically deleted when no CNI network is in use.

Podman then allocates a CNI netns in the infra container, by executing an equivalent of:
`podman exec rootless-cni-infra rootless-cni-infra alloc $CONTAINER_ID $NETWORK_NAME $POD_NAME`.

The allocated netns is deallocated when the container is being removed, by executing an equivalent of:
`podman exec rootless-cni-infra rootless-cni-infra dealloc $CONTAINER_ID $NETWORK_NAME`.

The container images live on `quay.io/libpod/rootless-cni-infra`.  The tags have the format `$version-$architecture`.  Please make sure to increase the version number in the Containerfile (i.e., `ROOTLESS_CNI_INFRA_VERSION`) when applying changes to this directory.  After committing the changes, upload the image(s) with the corresponding tag.

## Directory layout

* `/run/rootless-cni-infra/${CONTAINER_ID}/pid`: PID of the `sleep infinity` process that corresponds to the allocated netns
* `/run/rootless-cni-infra/${CONTAINER_ID}/attached/${NETWORK_NAME}`: CNI result
* `/run/rootless-cni-infra/${CONTAINER_ID}/attached-args/${NETWORK_NAME}`: CNI args
