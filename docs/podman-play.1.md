% podman-play(1)

## NAME
podman\-container - play pods and containers based on a structured input file

## SYNOPSIS
**podman play** *subcommand*

## DESCRIPTION
The play command will recreate pods and containers based on the input from a structured (like YAML)
file input.  Containers will be automatically started.

## COMMANDS

| Command  | Man Page                                            | Description                                                                  |
| -------  | --------------------------------------------------- | ---------------------------------------------------------------------------- |
| kube | [podman-play-kube(1)](podman-play-kube.1.md)              | Recreate pods and containers based on Kubernetes YAML.

## SEE ALSO
podman, podman-pod(1), podman-container(1), podman-generate(1), podman-play(1), podman-play-kube(1)
