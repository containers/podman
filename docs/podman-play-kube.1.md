% podman-play-kube Podman Man Pages
% Brent Baude
% December 2018
# NAME
podman-play-kube - Create pods and containers based on Kubernetes YAML

# SYNOPSIS
**podman play kube **
[**-h**|**--help**]
[**--authfile**]
[**--cert-dir**]
[**--creds**]
[***-q** | **--quiet**]
[**--signature-policy**]
[**--tls-verify**]
kubernetes_input.yml

# DESCRIPTION
**podman play kube** will read in a structured file of Kubernetes YAML.  It will then recreate
the pod and containers described in the YAML.  The containers within the pod are then started and
the ID of the new Pod is output.

Ideally the input file would be one created by Podman.  This would guarantee a smooth import and expected results.

# OPTIONS:

**--authfile**

Path of the authentication file. Default is ${XDG_RUNTIME\_DIR}/containers/auth.json, which is set using `podman login`.
If the authorization state is not found there, $HOME/.docker/config.json is checked, which is set using `docker login`.

Note: You can also override the default path of the authentication file by setting the REGISTRY\_AUTH\_FILE
environment variable. `export REGISTRY_AUTH_FILE=path`

**--cert-dir** *path*

Use certificates at *path* (\*.crt, \*.cert, \*.key) to connect to the registry.
Default certificates directory is _/etc/containers/certs.d_.

**--creds**

The [username[:password]] to use to authenticate with the registry if required.
If one or both values are not supplied, a command line prompt will appear and the
value can be entered.  The password is entered without echo.

**--quiet, -q**

Suppress output information when pulling images

**--signature-policy="PATHNAME"**

Pathname of a signature policy file to use.  It is not recommended that this
option be used, as the default behavior of using the system-wide default policy
(frequently */etc/containers/policy.json*) is most often preferred.

**--tls-verify**

Require HTTPS and verify certificates when contacting registries (default: true). If explicitly set to true,
then TLS verification will be used. If set to false, then TLS verification will not be used. If not specified,
TLS verification will be used unless the target registry is listed as an insecure registry in registries.conf.

**--help**, **-h**

Print usage statement

## Examples ##

Recreate the pod and containers as described in a file called `demo.yml`
```
$ podman play kube demo.yml
52182811df2b1e73f36476003a66ec872101ea59034ac0d4d3a7b40903b955a6
```

## SEE ALSO
podman(1), podman-container(1), podman-pod(1), podman-generate(1), podman-play(1)

# HISTORY
Decemeber 2018, Originally compiled by Brent Baude (bbaude at redhat dot com)
