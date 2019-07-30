% podman-login(1)

## NAME
podman\-login - Login to a container registry

## SYNOPSIS
**podman login** [*options*] *registry*

## DESCRIPTION
**podman login** logs into a specified registry server with the correct username
and password. **podman login** reads in the username and password from STDIN.
The username and password can also be set using the **username** and **password** flags.
The authentication storage can be set by the **authfile** option.
By default, the authentication storage is the kernel keyring. If the system does not
support kernel keyring, Podman will use the authentication file.
The path of the authentication file can be specified by the user by setting the **authfile** option
or REGISTRY\_AUTH\_FILE environment variable. The default path used is **${XDG\_RUNTIME_DIR}/containers/auth.json**.
If the **authfile** is set to "nokeyring", Podman will use the default authentication path or the
file set by REGISTRY\_AUTH\_FILE environment variable.

**podman [GLOBAL OPTIONS]**

**podman login [GLOBAL OPTIONS]**

**podman login [OPTIONS] REGISTRY [GLOBAL OPTIONS]**

## OPTIONS

**--password**, **-p**=*password*

Password for registry

**--password-stdin**

Take the password from stdin

**--username**, **-u=***username*

Username for registry

**--authfile**=*path*

Path of the authentication file or "nokeyring". By default, the authentication storage is the kernel keyring. If the system does not support kernel keyring, Podman will use the authentication file.  If **authfile** is set to "nokeyring", Podman will use the default authentication file ${XDG_\RUNTIME\_DIR}/containers/auth.json or the value of REGISTRY\_AUTH\_FILE environment variable (Not available for remote commands)

Note: The default path of the authentication file can also be overridden by setting the REGISTRY_AUTH_FILE environment variable. `export REGISTRY_AUTH_FILE=path`

**--get-login**

Return the logged-in user for the registry.  Return error if no login is found.

**--cert-dir**=*path*

Use certificates at *path* (\*.crt, \*.cert, \*.key) to connect to the registry.
Default certificates directory is _/etc/containers/certs.d_. (Not available for remote commands)

**--tls-verify**=*true|false*

Require HTTPS and verify certificates when contacting registries (default: true). If explicitly set to true,
then TLS verification will be used. If set to false, then TLS verification will not be used. If not specified,
TLS verification will be used unless the target registry is listed as an insecure registry in registries.conf. (Not available for remote commands)

**--help**, **-h**

Print usage statement

## EXAMPLES

```
$ podman login docker.io
Username: umohnani
Password:
Login Succeeded!
```

```
$ podman login -u testuser -p testpassword localhost:5000
Login Succeeded!
```

```
$ podman login --authfile authdir/myauths.json docker.io
Username: umohnani
Password:
Login Succeeded!
```

```
$ podman login --tls-verify=false -u test -p test localhost:5000
Login Succeeded!
```

```
$ podman login --cert-dir /etc/containers/certs.d/ -u foo -p bar localhost:5000
Login Succeeded!
```

```
$ podman login -u testuser  --password-stdin < testpassword.txt docker.io
Login Succeeded!
```

```
$ echo $testpassword | podman login -u testuser --password-stdin docker.io
Login Succeeded!
```

## SEE ALSO
podman(1), podman-logout(1)

## HISTORY
August 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
