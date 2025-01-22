% podman-artifact-inspect 1


## WARNING: Experimental command
*This command is considered experimental and still in development. Inputs, options, and outputs are all
subject to change.*

## NAME
podman\-artifact\-inspect - Inspect an OCI artifact

## SYNOPSIS
**podman artifact inspect** [*name*] ...

## DESCRIPTION

Inspect an artifact in the local store.  The artifact can be referred to with either:

1. Fully qualified artifact name
2. Full or partial digest of the artifact's manifest

## OPTIONS

#### **--help**

Print usage statement.

## EXAMPLES

Inspect an OCI image in the local store.
```
$ podman artifact inspect quay.io/myartifact/myml:latest
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-artifact(1)](podman-artifact.1.md)**

## HISTORY
Sept 2024, Originally compiled by Brent Baude <bbaude@redhat.com>
