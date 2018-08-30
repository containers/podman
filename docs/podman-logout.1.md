% podman-logout(1)

## NAME
podman\-logout - Logout of a container registry

## SYNOPSIS
**podman logout** [*options*] *registry*

## DESCRIPTION
**podman logout** logs out of a specified registry server by deleting the cached credentials
stored in the **auth.json** file. The path of the authentication file can be overridden by the user by setting the **authfile** flag.
The default path used is **${XDG\_RUNTIME_DIR}/containers/auth.json**.
All the cached credentials can be removed by setting the **all** flag.

**podman [GLOBAL OPTIONS]**

**podman logout [GLOBAL OPTIONS]**

**podman logout [OPTIONS] REGISTRY [GLOBAL OPTIONS]**

## OPTIONS

**--authfile**

Path of the authentication file. Default is ${XDG_\RUNTIME\_DIR}/containers/auth.json

**--all, -a**

Remove the cached credentials for all registries in the auth file

**--help**, **-h**

Print usage statement

## EXAMPLES

```
# podman logout docker.io
Remove login credentials for https://registry-1.docker.io/v2/
```

```
# podman logout --authfile authdir/myauths.json docker.io
Remove login credentials for https://registry-1.docker.io/v2/
```

```
# podman logout --all
Remove login credentials for all registries
```

## SEE ALSO
podman(1), podman-login(1), crio(8)

## HISTORY
August 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
