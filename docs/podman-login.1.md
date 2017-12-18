% podman(1) podman-login - Simple tool to login to a registry server
% Urvashi Mohnani
# podman-login "1" "August 2017" "podman"

## NAME
podman-login - Login to a container registry

## SYNOPSIS
**podman login**
[**--help**|**-h**]
[**--authfile**]
[**--user**|**-u**]
[**--password**|**-p**]
**REGISTRY**

## DESCRIPTION
**podman login** logs into a specified registry server with the correct username
and password. **podman login** reads in the username and password from STDIN.
The username and password can also be set using the **username** and **password** flags.
The path of the authentication file can be specified by the user by setting the **authfile**
flag. The default path used is **${XDG\_RUNTIME_DIR}/containers/auth.json**.

**podman [GLOBAL OPTIONS]**

**podman login [GLOBAL OPTIONS]**

**podman login [OPTIONS] REGISTRY [GLOBAL OPTIONS]**

## OPTIONS

**--password, -p**
Password for registry

**--username, -u**
Username for registry

**--authfile**
Path of the authentication file. Default is ${XDG_\RUNTIME\_DIR}/containers/auth.json

## EXAMPLES

```
# podman login docker.io
Username: umohnani
Password:
Login Succeeded!
```

```
# podman login -u testuser -p testpassword localhost:5000
Login Succeeded!
```

```
# podman login --authfile authdir/myauths.json docker.io
Username: umohnani
Password:
Login Succeeded!
```

## SEE ALSO
podman(1), podman-logout(1), crio(8), crio.conf(5)

## HISTORY
August 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
