# podman - Simple debugging tool for pods and images
podman is a simple client only tool to help with debugging issues when daemons such as CRI runtime and the kubelet are not responding or
failing. A shared API layer could be created to share code between the daemon and podman. podman does not require any daemon running. podman
utilizes the same underlying components that crio uses i.e. containers/image, container/storage, oci-runtime-tool/generate, runc or
any other OCI compatible runtime. podman shares state with crio and so has the capability to debug pods/images created by crio.

## Use cases
1. List pods.
2. Launch simple pods (that require no daemon support).
3. Exec commands in a container in a pod.
4. Launch additional containers in a pod.
5. List images.
6. Remove images not in use.
7. Pull images.
8. Check image size.
9. Report pod disk resource usage.
