% podman-generate-spec 1

## NAME
podman\-generate\-spec - Generate Specgen JSON based on containers or pods

## SYNOPSIS
**podman generate spec** [*options*] *container | *pod*

## DESCRIPTION
**podman generate spec** will generate Specgen JSON from Podman Containers and Pods. This JSON can either be printed to a file, directly to the command line, or both.

This JSON can then be used as input for the Podman API, specifically for Podman container and pod creation. Specgen is Podman's internal structure for formulating new container-related entities.

## OPTIONS

#### **--compact**, **-c**

Print the output in a compact, one line format. This is useful when piping the data to the Podman API

#### **--filename**, **-f**=**filename**

Output to the given file.

#### **--name**, **-n**

Rename the pod or container, so that it does not conflict with the existing entity. This is helpful when the JSON is to be used before the source pod or container is deleted.
