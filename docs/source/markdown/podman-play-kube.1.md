% podman-play-kube(1)

## NAME
podman-play-kube - Create containers, pods or volumes based on Kubernetes YAML

## SYNOPSIS
**podman play kube** [*options*] *file.yml|-*

## DESCRIPTION
**podman play kube** will read in a structured file of Kubernetes YAML.  It will then recreate the containers, pods or volumes described in the YAML.  Containers within a pod are then started and the ID of the new Pod or the name of the new Volume is output. If the yaml file is specified as "-" then `podman play kube` will read the YAML file from stdin.

Ideally the input file would be one created by Podman (see podman-generate-kube(1)).  This would guarantee a smooth import and expected results.

Currently, the supported Kubernetes kinds are:
- Pod
- Deployment
- PersistentVolumeClaim

`Kubernetes Pods or Deployments`

Only two volume types are supported by play kube, the *hostPath* and *persistentVolumeClaim* volume types. For the *hostPath* volume type, only the  *default (empty)*, *DirectoryOrCreate*, *Directory*, *FileOrCreate*, *File*, and *Socket* subtypes are supported. The *CharDevice* and *BlockDevice* subtypes are not supported. Podman interprets the value of *hostPath* *path* as a file path when it contains at least one forward slash, otherwise Podman treats the value as the name of a named volume. When using a *persistentVolumeClaim*, the value for *claimName* is the name for the Podman named volume.

Note: *hostPath* volume types created by play kube will be given an SELinux private label (Z)

Note: If the `:latest` tag is used, Podman will attempt to pull the image from a registry. If the image was built locally with Podman or Buildah, it will have `localhost` as the domain, in that case, Podman will use the image from the local store even if it has the `:latest` tag.

`Kubernetes PersistentVolumeClaims`

A Kubernetes PersistentVolumeClaim represents a Podman named volume. Only the PersistentVolumeClaim name is required by Podman to create a volume. Kubernetes annotations can be used to make use of the available options for Podman volumes.

- volume.podman.io/driver
- volume.podman.io/device
- volume.podman.io/type
- volume.podman.io/uid
- volume.podman.io/gid
- volume.podman.io/mount-options

## OPTIONS

#### **--authfile**=*path*

Path of the authentication file. Default is ${XDG\_RUNTIME\_DIR}/containers/auth.json, which is set using `podman login`.
If the authorization state is not found there, $HOME/.docker/config.json is checked, which is set using `docker login`.

Note: You can also override the default path of the authentication file by setting the REGISTRY\_AUTH\_FILE
environment variable. `export REGISTRY_AUTH_FILE=path`

#### **--cert-dir**=*path*

Use certificates at *path* (\*.crt, \*.cert, \*.key) to connect to the registry.
Please refer to containers-certs.d(5) for details. (This option is not available with the remote Podman client)

#### **--configmap**=*path*

Use Kubernetes configmap YAML at path to provide a source for environment variable values within the containers of the pod.

Note: The *--configmap* option can be used multiple times or a comma-separated list of paths can be used to pass multiple Kubernetes configmap YAMLs.

#### **--creds**

The [username[:password]] to use to authenticate with the registry if required.
If one or both values are not supplied, a command line prompt will appear and the
value can be entered.  The password is entered without echo.

#### **--ip**=*IP address*

Assign a static ip address to the pod. This option can be specified several times when play kube creates more than one pod.

#### **--log-driver**=driver

Set logging driver for all created containers.

#### **--mac-address**=*MAC address*

Assign a static mac address to the pod. This option can be specified several times when play kube creates more than one pod.

#### **--network**=*networks*, **--net**

A comma-separated list of the names of CNI networks the pod should join.

#### **--quiet**, **-q**

Suppress output information when pulling images

#### **--seccomp-profile-root**=*path*

Directory path for seccomp profiles (default: "/var/lib/kubelet/seccomp"). (This option is not available with the remote Podman client)

#### **--start**=*true|false*

Start the pod after creating it, set to false to only create it.

#### **--tls-verify**=*true|false*

Require HTTPS and verify certificates when contacting registries (default: true). If explicitly set to true,
then TLS verification will be used. If set to false, then TLS verification will not be used. If not specified,
TLS verification will be used unless the target registry is listed as an insecure registry in registries.conf.

#### **--help**, **-h**

Print usage statement

## EXAMPLES

Recreate the pod and containers as described in a file called `demo.yml`
```
$ podman play kube demo.yml
52182811df2b1e73f36476003a66ec872101ea59034ac0d4d3a7b40903b955a6
```

Recreate the pod and containers as described in a file `demo.yml` sent to stdin
```
$ cat demo.yml | podman play kube -
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
podman(1), podman-container(1), podman-pod(1), podman-generate-kube(1), podman-play(1), podman-network-create(1), containers-certs.d(5)

## HISTORY
December 2018, Originally compiled by Brent Baude (bbaude at redhat dot com)
