% podman-artifact-add 1

## WARNING: Experimental command
*This command is considered experimental and still in development. Inputs, options, and outputs are all
subject to change.*

## NAME
podman\-artifact\-add - Add an OCI artifact to the local store

## SYNOPSIS
**podman artifact add** *name* *file* [*file*]...

## DESCRIPTION

Add an OCI artifact to the local store from the local filesystem.  You must
provide at least one file to create the artifact, but several can also be
added.


## OPTIONS

#### **--help**

Print usage statement.


## EXAMPLES

Add a single file to an artifact

```
$ podman artifact add quay.io/myartifact/myml:latest /tmp/foobar.ml
0fe1488ecdef8cc4093e11a55bc048d9fc3e13a4ba846efd24b5a715006c95b3
```

Add multiple files to an artifact
```
$ podman artifact add quay.io/myartifact/myml:latest /tmp/foobar1.ml /tmp/foobar2.ml
1487acae11b5a30948c50762882036b41ac91a7b9514be8012d98015c95ddb78
```



## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-artifact(1)](podman-artifact.1.md)**

## HISTORY
Jan 2025, Originally compiled by Brent Baude <bbaude@redhat.com>
