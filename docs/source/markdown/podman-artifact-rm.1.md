% podman-artifact-rm 1


## WARNING: Experimental command
*This command is considered experimental and still in development. Inputs, options, and outputs are all
subject to change.*

## NAME
podman\-artifact\-rm - Remove an OCI from local storage

## SYNOPSIS
**podman artifact rm** [*options*] *name*

## DESCRIPTION

Remove an artifact from the local artifact store.  The input may be the fully
qualified artifact name or a full or partial artifact digest.

## OPTIONS

#### **--all**, **-a**

Remove all artifacts in the local store.  The use of this option conflicts with
providing a name or digest of the artifact.

#### **--help**

Print usage statement.


## EXAMPLES

Remove an artifact by name

```
$ podman artifact rm quay.io/artifact/foobar2:test
Deleted: e7b417f49fc24fc7ead6485da0ebd5bc4419d8a3f394c169fee5a6f38faa4056
```

Remove an artifact by partial digest

```
$ podman artifact rm e7b417f49fc
Deleted: e7b417f49fc24fc7ead6485da0ebd5bc4419d8a3f394c169fee5a6f38faa4056
```

Remove all artifacts in local storage
```
$ podman artifact rm -a
Deleted: cee15f7c5ce3e86ae6ce60d84bebdc37ad34acfa9a2611cf47501469ac83a1ab
Deleted: 72875f8f6f78d5b8ba98b2dd2c0a6f395fde8f05ff63a1df580d7a88f5afa97b
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-artifact(1)](podman-artifact.1.md)**

## HISTORY
Jan 2025, Originally compiled by Brent Baude <bbaude@redhat.com>
