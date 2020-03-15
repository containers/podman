% podman-play-kube(1)

## NAME
podman-play-kube - Create pods and containers based on Kubernetes YAML

## SYNOPSIS
**podman play kube** [*options*] *file*__.yml__

## DESCRIPTION
**podman play kube** will read in a structured file of Kubernetes YAML.  It will then recreate
the pod and containers described in the YAML.  The containers within the pod are then started and
the ID of the new Pod is output.

Ideally the input file would be one created by Podman (see podman-generate-kube(1)).  This would guarantee a smooth import and expected results.

Note: HostPath volume types created by play kube will be given an SELinux private label (Z)

## OPTIONS:

**--authfile**=*path*

Path of the authentication file. Default is ${XDG\_RUNTIME\_DIR}/containers/auth.json, which is set using `podman login`.
If the authorization state is not found there, $HOME/.docker/config.json is checked, which is set using `docker login`. (Not available for remote commands)

Note: You can also override the default path of the authentication file by setting the REGISTRY\_AUTH\_FILE
environment variable. `export REGISTRY_AUTH_FILE=path`

**--cert-dir**=*path*

Use certificates at *path* (\*.crt, \*.cert, \*.key) to connect to the registry.
Default certificates directory is _/etc/containers/certs.d_. (Not available for remote commands)

**--creds**

The [username[:password]] to use to authenticate with the registry if required.
If one or both values are not supplied, a command line prompt will appear and the
value can be entered.  The password is entered without echo.

**--quiet**, **-q**

Suppress output information when pulling images

**--seccomp-profile-root**=*path*

Directory path for seccomp profiles (default: "/var/lib/kubelet/seccomp"). (Not available for remote commands)

**--tls-verify**=*true|false*

Require HTTPS and verify certificates when contacting registries (default: true). If explicitly set to true,
then TLS verification will be used. If set to false, then TLS verification will not be used. If not specified,
TLS verification will be used unless the target registry is listed as an insecure registry in registries.conf. (Not available for remote commands)

**--help**, **-h**

Print usage statement

## Examples

Recreate the pod and containers as described in a file called `demo.yml`
```
$ podman play kube demo.yml
52182811df2b1e73f36476003a66ec872101ea59034ac0d4d3a7b40903b955a6
```

## SEE ALSO
podman(1), podman-container(1), podman-pod(1), podman-generate-kube(1), podman-play(1)

## HISTORY
December 2018, Originally compiled by Brent Baude (bbaude at redhat dot com)
