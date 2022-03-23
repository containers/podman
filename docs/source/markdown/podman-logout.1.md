% podman-logout(1)

## NAME
podman\-logout - Logout of a container registry

## SYNOPSIS
**podman logout** [*options*] *registry*

## DESCRIPTION
**podman logout** logs out of a specified registry server by deleting the cached credentials
stored in the **auth.json** file. If the registry is not specified, the first registry under [registries.search]
from registries.conf will be used. The path of the authentication file can be overridden by the user by setting the **authfile** flag.
The default path used is **${XDG\_RUNTIME\_DIR}/containers/auth.json**. For more details about format and configurations of the auth,json file, please refer to containers-auth.json(5)
All the cached credentials can be removed by setting the **all** flag.

**podman [GLOBAL OPTIONS]**

**podman logout [GLOBAL OPTIONS]**

**podman logout [OPTIONS] REGISTRY [GLOBAL OPTIONS]**

## OPTIONS

#### **--all**, **-a**

Remove the cached credentials for all registries in the auth file

#### **--authfile**=*path*

Path of the authentication file. Default is ${XDG\_RUNTIME\_DIR}/containers/auth.json.

Note: You can also override the default path of the authentication file by setting the REGISTRY\_AUTH\_FILE
environment variable. `export REGISTRY_AUTH_FILE=path`

#### **--help**, **-h**

Print usage statement

## EXAMPLES

```
$ podman logout docker.io
Remove login credentials for https://registry-1.docker.io/v2/
```

```
$ podman logout --authfile authdir/myauths.json docker.io
Remove login credentials for https://registry-1.docker.io/v2/
```

```
$ podman logout --all
Remove login credentials for all registries
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-login(1)](podman-login.1.md)**, **[containers-auth.json(5)](https://github.com/containers/image/blob/main/docs/containers-auth.json.5.md)**

## HISTORY
August 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
