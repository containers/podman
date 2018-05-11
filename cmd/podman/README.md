# podman - Simple debugging tool for pods and images
podman is a daemonless container runtime for managing containers, pods, and container images.
It is intended as a counterpart to CRI-O, to provide low-level debugging not available through the CRI interface used by Kubernetes.
It can also act as a container runtime independent of CRI-O, creating and managing its own set of containers.

## Use cases
1. Create containers
2. Start, stop, signal, attach to, and inspect existing containers
3. Run new commands in existing containers
4. Push and pull images
5. List and inspect existing images
6. Create new images by committing changes within a container
7. Create pods
8. Start, stop, signal, and inspect existing pods
9. Populate pods with containers
