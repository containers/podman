![PODMAN logo](../../logo/podman-logo-source.svg)

A container image for tracking automation metadata.
Currently this is used to update last-used timestamps on
VM images.

Example build (from repository root):

```bash
sudo podman build -t $IMAGE_NAME -f contrib/imgts/Dockerfile .
```
