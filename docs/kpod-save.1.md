% kpod(1) kpod-save - Simple tool to save an image to an archive
% Urvashi Mohnani
# kpod-save "1" "July 2017" "kpod"

## NAME
kpod-save - Save an image to docker-archive or oci-archive

## SYNOPSIS
**kpod save**
**NAME[:TAG]**
[**--quiet**|**-q**]
[**--format**]
[**--output**|**-o**]
[**--help**|**-h**]

## DESCRIPTION
**kpod save** saves an image to either **docker-archive** or **oci-archive**
on the local machine, default is **docker-archive**.
**kpod save** writes to STDOUT by default and can be redirected to a file using the **output** flag.
The **quiet** flag suppresses the output when set.

**kpod [GLOBAL OPTIONS]**

**kpod save [GLOBAL OPTIONS]**

**kpod save [OPTIONS] NAME[:TAG]**

## OPTIONS

**--output, -o**
Write to a file, default is STDOUT

**--format**
Save image to **oci-archive**
```
--format oci-archive
```

**--quiet, -q**
Suppress the output

## EXAMPLES

```
# kpod save --quiet -o alpine.tar alpine:2.6
```

```
# kpod save > alpine-all.tar alpine
```

```
# kpod save -o oci-alpine.tar --format oci-archive alpine
```

## SEE ALSO
kpod(1), kpod-load(1), crio(8), crio.conf(5)

## HISTORY
July 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
