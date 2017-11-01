% kpod(1) kpod-export - Simple tool to export a container's filesystem as a tarball
% Urvashi Mohnani
# kpod-export "1" "July 2017" "kpod"

## NAME
kpod-export - Export container's filesystem contents as a tar archive

## SYNOPSIS
**kpod export**
**CONTAINER**
[**--output**|**-o**]
[**--help**|**-h**]

## DESCRIPTION
**kpod export** exports the filesystem of a container and saves it as a tarball
on the local machine. **kpod export** writes to STDOUT by default and can be
redirected to a file using the **output flag**.

**kpod [GLOBAL OPTIONS]**

**kpod export [GLOBAL OPTIONS]**

**kpod export [OPTIONS] CONTAINER**

## OPTIONS

**--output, -o**
Write to a file, default is STDOUT

## EXAMPLES

```
# kpod export -o redis-container.tar 883504668ec465463bc0fe7e63d53154ac3b696ea8d7b233748918664ea90e57
```

```
# kpod export > redis-container.tar 883504668ec465463bc0fe7e63d53154ac3b696ea8d7b233748918664ea90e57
```

## SEE ALSO
kpod(1), kpod-import(1), crio(8), crio.conf(5)

## HISTORY
August 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
