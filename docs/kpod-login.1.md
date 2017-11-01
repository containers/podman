% kpod(1) kpod-login - Simple tool to login to a registry server
% Urvashi Mohnani
# kpod-login "1" "August 2017" "kpod"

## NAME
kpod-login - Login to a container registry

## SYNOPSIS
**kpod login**
[**--help**|**-h**]
[**--authfile**]
[**--user**|**-u**]
[**--password**|**-p**]
**REGISTRY**

## DESCRIPTION
**kpod login** logs into a specified registry server with the correct username
and password. **kpod login** reads in the username and password from STDIN.
The username and password can also be set using the **username** and **password** flags.
The path of the authentication file can be specified by the user by setting the **authfile**
flag. The default path used is **${XDG\_RUNTIME_DIR}/containers/auth.json**.

**kpod [GLOBAL OPTIONS]**

**kpod login [GLOBAL OPTIONS]**

**kpod login [OPTIONS] REGISTRY [GLOBAL OPTIONS]**

## OPTIONS

**--password, -p**
Password for registry

**--username, -u**
Username for registry

**--authfile**
Path of the authentication file. Default is ${XDG_\RUNTIME\_DIR}/containers/auth.json

## EXAMPLES

```
# kpod login docker.io
Username: umohnani
Password:
Login Succeeded!
```

```
# kpod login -u testuser -p testpassword localhost:5000
Login Succeeded!
```

```
# kpod login --authfile authdir/myauths.json docker.io
Username: umohnani
Password:
Login Succeeded!
```

## SEE ALSO
kpod(1), kpod-logout(1), crio(8), crio.conf(5)

## HISTORY
August 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
