% podman-image-scan(1)

## NAME
podman\-image\-scan - Scan an image's contents

## SYNOPSIS
**podman image scan** [*options*] *name*[:*tag*] -- [*scanner options*]

## DESCRIPTION
Scans the contents of an image with a pluggable scanner tool. All stdout and stderr from the scanner tool is written to
stderr and stdout of **podman image scan**. The tool exits with a non-zero code if the scan fails.

Any arguments after `--` are provided to the scanner tool. Templating is performed against the scanner tool arguments
to allow for providing the mount point of the target image to be scanned to the scanner tool. Specifically all references
of `{{ .Mountpoint }}` are replaced with what the user provided with `--mount-point`/`-m`, or if not provided, the
auto-generated mount-point is provided.

**podman image scan [OPTIONS] NAME[:TAG] -- [SCANNER OPTIONS]**

## RETURN VALUE
The scanner tool report.

## OPTIONS

#### **--env**, **-e**=*env*

Set environment variables within the scanner tool container.

This option allows arbitrary environment variables that are available for the process to be launched inside of the container.
If an environment variable is specified without a value, Podman will check the host environment for a value and set the variable
only if it is set on the host. If an environment variable ending in __*__ is specified, Podman will search the host environment
for variables starting with the prefix and will add those variables to the container. If an environment variable with a
trailing ***** is specified, then a value must be supplied.

#### **--mount-point**, **-m** [*path*]

The volume mount path of the target image as presented to the scanner tool container.


#### **--scanner**, **-s** [*image*]

A container image with the scanner tool installed and configured as the entrypoint. The scanner tool should exit with
a non-zero code to indicate a failing scan.

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
podman image scan alpine:latest -- 'dir:{{ .Mountpoint }}' -o json -vv
```

Change the image format presented to the scanner:
```
podman image scan --format docker-archive alpine:latest -- 'docker-archive:{{ .Mountpoint }}' -o json -vv
```

## SEE ALSO
podman(1)
