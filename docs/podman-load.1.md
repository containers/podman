% podman-load(1)

## NAME
podman\-load - Load an image from docker archive

## SYNOPSIS
**podman load** *name*[:*tag*|@*digest*]

## DESCRIPTION
**podman load** copies an image from either **docker-archive** or **oci-archive** stored
on the local machine. **podman load** reads from stdin by default or a file if the **input** flag is set.
The **quiet** flag suppresses the output when set.
Note: `:` is a restricted character and cannot be part of the file name.

**podman [GLOBAL OPTIONS]**

**podman load [GLOBAL OPTIONS]**

**podman load [OPTIONS] NAME[:TAG|@DIGEST]**

## OPTIONS

**--input, -i**

Read from archive file, default is STDIN

**--quiet, -q**

Suppress the output

**--signature-policy="PATHNAME"**

Pathname of a signature policy file to use.  It is not recommended that this
option be used, as the default behavior of using the system-wide default policy
(frequently */etc/containers/policy.json*) is most often preferred

**--help**, **-h**

Print usage statement

## EXAMPLES

```
# podman load --quiet -i fedora.tar
```

```
# podman load -q --signature-policy /etc/containers/policy.json -i fedora.tar
```

```
# podman load < fedora.tar
Getting image source signatures
Copying blob sha256:5bef08742407efd622d243692b79ba0055383bbce12900324f75e56f589aedb0
 0 B / 4.03 MB [---------------------------------------------------------------]
Copying config sha256:7328f6f8b41890597575cbaadc884e7386ae0acc53b747401ebce5cf0d624560
 0 B / 1.48 KB [---------------------------------------------------------------]
Writing manifest to image destination
Storing signatures
Loaded image:  registry.fedoraproject.org/fedora:latest
```

```
# cat fedora.tar | podman load
Getting image source signatures
Copying blob sha256:5bef08742407efd622d243692b79ba0055383bbce12900324f75e56f589aedb0
 0 B / 4.03 MB [---------------------------------------------------------------]
Copying config sha256:7328f6f8b41890597575cbaadc884e7386ae0acc53b747401ebce5cf0d624560
 0 B / 1.48 KB [---------------------------------------------------------------]
Writing manifest to image destination
Storing signatures
Loaded image:  registry.fedoraproject.org/fedora:latest
```

## SEE ALSO
podman(1), podman-save(1), crio(8)

## HISTORY
July 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
