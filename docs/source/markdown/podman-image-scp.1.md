% podman-image-scp(1)

## NAME
podman-image-scp - Securely copy an image from one host to another

## SYNOPSIS
**podman image scp** [*options*] *name*[:*tag*]

## DESCRIPTION
**podman image scp** copies container images between hosts on a network. You can load to the remote host or from the remote host as well as in between two remote hosts.
Note: `::` is used to specify the image name depending on if you are saving or loading. Images can also be transferred from rootful to rootless storage on the same machine without using sshd. This feature is not supported on the remote client, including Mac and Windows (excluding WSL2) machines.

**podman image scp [GLOBAL OPTIONS]**

**podman image** *scp [OPTIONS] NAME[:TAG] [HOSTNAME::]*

**podman image** *scp [OPTIONS] [HOSTNAME::]IMAGENAME*

**podman image** *scp [OPTIONS] [HOSTNAME::]IMAGENAME [HOSTNAME::]*

## OPTIONS

#### **--help**, **-h**

Print usage statement

#### **--quiet**, **-q**

Suppress the output

## EXAMPLES


```
$ podman image scp alpine
Loaded image(s): docker.io/library/alpine:latest
```

```
$ podman image scp alpine Fedora::/home/charliedoern/Documents/alpine
Getting image source signatures
Copying blob 72e830a4dff5 done
Copying config 85f9dc67c7 done
Writing manifest to image destination
Storing signatures
Loaded image(s): docker.io/library/alpine:latest
```

```
$ podman image scp Fedora::alpine RHEL::
Loaded image(s): docker.io/library/alpine:latest
```

```
$ podman image scp charliedoern@192.168.68.126:22/run/user/1000/podman/podman.sock::alpine
WARN[0000] Unknown connection name given. Please use system connection add to specify the default remote socket location
Getting image source signatures
Copying blob 9450ef9feb15 [--------------------------------------] 0.0b / 0.0b
Copying config 1f97f0559c done
Writing manifest to image destination
Storing signatures
Loaded image(s): docker.io/library/alpine:latest
```

```
$ sudo podman image scp root@localhost::alpine username@localhost::
Copying blob e2eb06d8af82 done
Copying config 696d33ca15 done
Writing manifest to image destination
Storing signatures
Getting image source signatures
Copying blob 5eb901baf107 skipped: already exists
Copying config 696d33ca15 done
Writing manifest to image destination
Storing signatures
Loaded image(s): docker.io/library/alpine:latest
```

```
$ sudo podman image scp root@localhost::alpine
Copying blob e2eb06d8af82 done
Copying config 696d33ca15 done
Writing manifest to image destination
Storing signatures
Getting image source signatures
Copying blob 5eb901baf107
Copying config 696d33ca15 done
Writing manifest to image destination
Storing signatures
Loaded image(s): docker.io/library/alpine:latest
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-load(1)](podman-load.1.md)**, **[podman-save(1)](podman-save.1.md)**, **[podman-remote(1)](podman-remote.1.md)**, **[podman-system-connection-add(1)](podman-system-connection-add.1.md)**, **[containers.conf(5)](https://github.com/containers/common/blob/main/docs/containers.conf.5.md)**, **[containers-transports(5)](https://github.com/containers/image/blob/main/docs/containers-transports.5.md)**

## HISTORY
July 2021, Originally written by Charlie Doern <cdoern@redhat.com>
