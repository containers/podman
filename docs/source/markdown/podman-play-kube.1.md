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

## OPTIONS

**--authfile**=*path*

Path of the authentication file. Default is ${XDG\_RUNTIME\_DIR}/containers/auth.json, which is set using `podman login`.
If the authorization state is not found there, $HOME/.docker/config.json is checked, which is set using `docker login`.

Note: You can also override the default path of the authentication file by setting the REGISTRY\_AUTH\_FILE
environment variable. `export REGISTRY_AUTH_FILE=path`

**--cert-dir**=*path*

Use certificates at *path* (\*.crt, \*.cert, \*.key) to connect to the registry.
Default certificates directory is _/etc/containers/certs.d_. (Not available for remote commands)

**--configmap**=*path*

Use Kubernetes configmap YAML at path to provide a source for environment variable values within the containers of the pod.

Note: The *--configmap* option can be used multiple times or a comma-separated list of paths can be used to pass multiple Kubernetes configmap YAMLs.

**--creds**

The [username[:password]] to use to authenticate with the registry if required.
If one or both values are not supplied, a command line prompt will appear and the
value can be entered.  The password is entered without echo.

**--network**=*cni networks*

A comma-separated list of the names of CNI networks the pod should join.

**--quiet**, **-q**

Suppress output information when pulling images

**--seccomp-profile-root**=*path*

Directory path for seccomp profiles (default: "/var/lib/kubelet/seccomp"). (Not available for remote commands)

**--tls-verify**=*true|false*

Require HTTPS and verify certificates when contacting registries (default: true). If explicitly set to true,
then TLS verification will be used. If set to false, then TLS verification will not be used. If not specified,
TLS verification will be used unless the target registry is listed as an insecure registry in registries.conf.

**--help**, **-h**

Print usage statement

## EXAMPLES

Recreate the pod and containers as described in a file called `demo.yml`
```
$ podman play kube demo.yml
52182811df2b1e73f36476003a66ec872101ea59034ac0d4d3a7b40903b955a6
```

Provide `configmap-foo.yml` and `configmap-bar.yml` as sources for environment variables within the containers.
```
$ podman play kube demo.yml --configmap configmap-foo.yml,configmap-bar.yml
52182811df2b1e73f36476003a66ec872101ea59034ac0d4d3a7b40903b955a6

$ podman play kube demo.yml --configmap configmap-foo.yml --configmap configmap-bar.yml
52182811df2b1e73f36476003a66ec872101ea59034ac0d4d3a7b40903b955a6
```

CNI network(s) can be specified as comma-separated list using ``--network``
```
$ podman play kube demo.yml --network cni1,cni2
52182811df2b1e73f36476003a66ec872101ea59034ac0d4d3a7b40903b955a6
```

Please take into account that CNI networks must be created first using podman-network-create(1).

## SEE ALSO
podman(1), podman-container(1), podman-pod(1), podman-generate-kube(1), podman-play(1), podman-network-create(1)

## HISTORY
December 2018, Originally compiled by Brent Baude (bbaude at redhat dot com)
