% podman-down-kube(1)

## NAME
podman-down-kube - Stop containers, pods or volumes based on Kubernetes YAML

## SYNOPSIS
**podman down kube** [*options*] *file.yml|-*

## DESCRIPTION
**podman down kube** will tear down the pods created by a previous run of `podman play kube`. It reads in a structured file of Kubernetes YAML.  It will then stop the containers, pods or volumes described in the YAML. If the yaml file is specified as "-" then `podman down kube` will read the YAML file from stdin.
Ideally the input file would be one created by Podman (see podman-generate-kube(1)).  This would guarantee a smooth import and expected results.

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-play(1)](podman-play.1.md)**, **[podman-play-kube(1)](podman-play-kube.1.md)**, **[podman-network-create(1)](podman-network-create.1.md)**, **[podman-generate-kube(1)](podman-generate-kube.1.md)**, **[podman-down(1)](podman-down.1.md)**
