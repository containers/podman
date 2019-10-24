% podman-generate(1)

## NAME
podman\-generate - Generate structured data based for a containers and pods

## SYNOPSIS
**podman generate** *subcommand*

## DESCRIPTION
The generate command will create structured output (like YAML) based on a container or pod.

## COMMANDS

| Command | Man Page                                                   | Description                                                                         |
|---------|------------------------------------------------------------|-------------------------------------------------------------------------------------|
| kube    | [podman-generate-kube(1)](podman-generate-kube.1.md)       | Generate Kubernetes YAML based on a pod or container.                               |
| systemd | [podman-generate-systemd(1)](podman-generate-systemd.1.md) | Generate systemd unit file(s) for a container. Not supported for the remote client. |


## SEE ALSO
podman, podman-pod, podman-container
