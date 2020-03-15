![PODMAN logo](../../logo/podman-logo-source.svg)

A container image for maintaining the collection of
VM images used by CI/CD on this project and several others.
Acts upon metadata maintained by the imgts container.

Example build (from repository root):

```bash
sudo podman build -t $IMAGE_NAME -f contrib/imgprune/Dockerfile .
```
