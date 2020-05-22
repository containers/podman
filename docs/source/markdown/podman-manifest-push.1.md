% podman-manifest-push(1)

## NAME
podman\-manifest\-push - Push a manifest list or image index to a registry

## SYNOPSIS
**podman manifest push** [options...] *listnameorindexname* *transport:details*

## DESCRIPTION
Pushes a manifest list or image index to a registry.

## RETURN VALUE
The list image's ID and the digest of the image's manifest.

## OPTIONS

**--all**

Push the images mentioned in the manifest list or image index, in addition to
the list or index itself.

**--authfile**=*path*

Path of the authentication file. Default is ${XDG\_RUNTIME\_DIR}/containers/auth.json, which is set using `podman login`.
If the authorization state is not found there, $HOME/.docker/config.json is checked, which is set using `docker login`. (Not available for remote commands)

Note: You can also override the default path of the authentication file by setting the REGISTRY\_AUTH\_FILE
environment variable. `export REGISTRY_AUTH_FILE=path`

**--cert-dir**=*path*

Use certificates at *path* (\*.crt, \*.cert, \*.key) to connect to the registry.
Default certificates directory is _/etc/containers/certs.d_. (Not available for remote commands)

**--creds**=*creds*

The [username[:password]] to use to authenticate with the registry if required.
If one or both values are not supplied, a command line prompt will appear and the
value can be entered.  The password is entered without echo.

**--digestfile**=*Digestfile*

After copying the image, write the digest of the resulting image to the file.

**--format**, **-f**=*format*

Manifest list type (oci or v2s2) to use when pushing the list (default is oci).

**--purge**

Delete the manifest list or image index from local storage if pushing succeeds.

**--quiet**, **-q**

When writing the manifest, suppress progress output

**--remove-signatures**

Don't copy signatures when pushing images.

**--sign-by**=*fingerprint*

Sign the pushed images using the GPG key that matches the specified fingerprint.

**--tls-verify**

Require HTTPS and verify certificates when talking to container registries (defaults to true) (Not available for remote commands)

## EXAMPLE

```
podman manifest push mylist:v1.11 docker://registry.example.org/mylist:v1.11
```

## SEE ALSO
podman(1), podman-manifest(1), podman-manifest-add(1), podman-manifest-create(1), podman-manifest-inspect(1), podman-manifest-remove(1), podman-rmi(1)
