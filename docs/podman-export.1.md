% podman-export "1"

## NAME
podman export - Export container's filesystem contents as a tar archive

## SYNOPSIS
**podman export**
**CONTAINER**
[**--output**|**-o**]
[**--help**|**-h**]

## DESCRIPTION
**podman export** exports the filesystem of a container and saves it as a tarball
on the local machine. **podman export** writes to STDOUT by default and can be
redirected to a file using the **output flag**.
Note: `:` is a restricted character and cannot be part of the file name.

**podman [GLOBAL OPTIONS]**

**podman export [GLOBAL OPTIONS]**

**podman export [OPTIONS] CONTAINER**

## OPTIONS

**--output, -o**

Write to a file, default is STDOUT

## EXAMPLES

```
# podman export -o redis-container.tar 883504668ec465463bc0fe7e63d53154ac3b696ea8d7b233748918664ea90e57
```

```
# podman export > redis-container.tar 883504668ec465463bc0fe7e63d53154ac3b696ea8d7b233748918664ea90e57
```

## SEE ALSO
podman(1), podman-import(1), crio(8)

## HISTORY
August 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
