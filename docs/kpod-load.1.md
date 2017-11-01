% kpod(1) kpod-load - Simple tool to load an image from an archive to containers-storage
% Urvashi Mohnani
# kpod-load "1" "July 2017" "kpod"

## NAME
kpod-load - Load an image from docker archive

## SYNOPSIS
**kpod load**
**NAME[:TAG|@DIGEST]**
[**--input**|**-i**]
[**--quiet**|**-q**]
[**--help**|**-h**]

## DESCRIPTION
**kpod load** copies an image from either **docker-archive** or **oci-archive** stored
on the local machine. **kpod load** reads from stdin by default or a file if the **input** flag is set.
The **quiet** flag suppresses the output when set.

**kpod [GLOBAL OPTIONS]**

**kpod load [GLOBAL OPTIONS]**

**kpod load [OPTIONS] NAME[:TAG|@DIGEST]**

## OPTIONS

**--input, -i**
Read from archive file, default is STDIN

**--quiet, -q**
Suppress the output

**--signature-policy="PATHNAME"**

Pathname of a signature policy file to use.  It is not recommended that this
option be used, as the default behavior of using the system-wide default policy
(frequently */etc/containers/policy.json*) is most often preferred

## EXAMPLES

```
# kpod load --quiet -i fedora.tar
```

```
# kpod load -q --signature-policy /etc/containers/policy.json -i fedora.tar
```

```
# kpod load < fedora.tar
Getting image source signatures
Copying blob sha256:5bef08742407efd622d243692b79ba0055383bbce12900324f75e56f589aedb0
 0 B / 4.03 MB [---------------------------------------------------------------]
Copying config sha256:7328f6f8b41890597575cbaadc884e7386ae0acc53b747401ebce5cf0d624560
 0 B / 1.48 KB [---------------------------------------------------------------]
Writing manifest to image destination
Storing signatures
```

```
# cat fedora.tar | kpod load
Getting image source signatures
Copying blob sha256:5bef08742407efd622d243692b79ba0055383bbce12900324f75e56f589aedb0
 0 B / 4.03 MB [---------------------------------------------------------------]
Copying config sha256:7328f6f8b41890597575cbaadc884e7386ae0acc53b747401ebce5cf0d624560
 0 B / 1.48 KB [---------------------------------------------------------------]
Writing manifest to image destination
Storing signatures
```

## SEE ALSO
kpod(1), kpod-save(1), crio(8), crio.conf(5)

## HISTORY
July 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
