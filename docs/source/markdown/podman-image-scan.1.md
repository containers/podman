% podman-image-scan(1)

## NAME
podman\-image\-scan - Scan an image's root filesystem for known vulnerabilities

## SYNOPSIS
**podman image scan** [*options*] [*image* ...] [*scanner options*]

## DESCRIPTION
Mounts the specified images' root file system and scans the contents accessible from
the mount point for known vulnerabilities.

## RETURN VALUE
The scanner tool report.

## OPTIONS

#### **--scanner**, **-s** [*image*]

A container image with the scanner tool installed and configured as the entrypoint and 
reads the PODMAN_SCAN_MOUNT environment variable to determine where to scan.

## EXAMPLE

Use the default scanner to scan alpine:latest
```
podman image scan alpine:latest

#... the scanner report is output to stdout / stderr>
```

Switch out the default scanner image for a custom tool:
```
podman image scan --scanner anchore/grype:latest alpine:latest
```

Pass arguments to the scanner:
```
podman image scan alpine:latest -- -o json -vv
```

## SEE ALSO
podman(1)
