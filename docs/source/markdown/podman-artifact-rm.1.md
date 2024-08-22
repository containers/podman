% podman-artifact-rm 1


## WARNING: Experimental command
*This command is considered experimental and still in development. Inputs, options, and outputs are all
subject to change.*

## NAME
podman\-artifact\-rm - Remove an OCI from local storage

## SYNOPSIS
**podman artifact rm** *name*

## DESCRIPTION

Remove an artifact from the local artifact store.  The input may be the fully
qualified artifact name or a full or partial artifact digest.

## OPTIONS

#### **--help**

Print usage statement.


## EXAMPLES

Remove an artifact by name

```
$ podman artifact rm quay.io/artifact/foobar2:test
e7b417f49fc24fc7ead6485da0ebd5bc4419d8a3f394c169fee5a6f38faa4056
```

Remove an artifact by partial digest

```
$ podman artifact rm e7b417f49fc
e7b417f49fc24fc7ead6485da0ebd5bc4419d8a3f394c169fee5a6f38faa4056
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-artifact(1)](podman-artifact.1.md)**

## HISTORY
Jan 2025, Originally compiled by Brent Baude <bbaude@redhat.com>
