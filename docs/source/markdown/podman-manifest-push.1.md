% podman-manifest-push(1)

## NAME
podman\-manifest\-push - Push a manifest list or image index to a registry

## SYNOPSIS
**podman manifest push** [*options*] *listnameorindexname* [*destination*]

## DESCRIPTION
Pushes a manifest list or image index to a registry.

## RETURN VALUE
The list image's ID and the digest of the image's manifest.

## OPTIONS

#### **--all**

Push the images mentioned in the manifest list or image index, in addition to
the list or index itself. (Default true)

#### **--authfile**=*path*

Path of the authentication file. Default is ${XDG\_RUNTIME\_DIR}/containers/auth.json, which is set using `podman login`.
If the authorization state is not found there, $HOME/.docker/config.json is checked, which is set using `docker login`.

Note: You can also override the default path of the authentication file by setting the REGISTRY\_AUTH\_FILE
environment variable. `export REGISTRY_AUTH_FILE=path`

#### **--cert-dir**=*path*

Use certificates at *path* (\*.crt, \*.cert, \*.key) to connect to the registry. (Default: /etc/containers/certs.d)
Please refer to containers-certs.d(5) for details. (This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)

#### **--creds**=*creds*

The [username[:password]] to use to authenticate with the registry if required.
If one or both values are not supplied, a command line prompt will appear and the
value can be entered.  The password is entered without echo.

#### **--digestfile**=*Digestfile*

After copying the image, write the digest of the resulting image to the file.

#### **--format**, **-f**=*format*

Manifest list type (oci or v2s2) to use when pushing the list (default is oci).

#### **--quiet**, **-q**

When writing the manifest, suppress progress output

#### **--remove-signatures**

Don't copy signatures when pushing images.

#### **--rm**

Delete the manifest list or image index from local storage if pushing succeeds.

#### **--sign-by**=*fingerprint*

Sign the pushed images using the GPG key that matches the specified fingerprint.

#### **--tls-verify**

Require HTTPS and verify certificates when talking to container registries. (defaults to true)

## DESTINATION

 The DESTINATION is a location to store container images
 The Image "DESTINATION" uses a "transport":"details" format.
 If a transport is not given, podman push will attempt to push
 to a registry.

 Multiple transports are supported:

  **dir:**_path_
  An existing local directory _path_ storing the manifest, layer tarballs and signatures as individual files. This is a non-standardized format, primarily useful for debugging or noninvasive container inspection.

    $ podman manifest push mylist:v1.11 dir:/tmp/mylist

  **docker://**_docker-reference_
  An image in a registry implementing the "Docker Registry HTTP API V2". By default, uses the authorization state in `$XDG_RUNTIME_DIR/containers/auth.json`, which is set using `(podman login)`. If the authorization state is not found there, `$HOME/.docker/config.json` is checked, which is set using `(docker login)`.

    $ podman manifest push mylist:v1.11 docker://registry.example.org/mylist:v1.11

  **docker-archive:**_path_[**:**_docker-reference_]
  An image is stored in the `docker save` formatted file.  _docker-reference_ is only used when creating such a file, and it must not contain a digest.

    $ podman manifest push mylist:v1.11 docker-archive:/tmp/mylist

  **docker-daemon:**_docker-reference_
  An image in _docker-reference_ format stored in the docker daemon internal storage. _docker-reference_ must contain a tag.

    $ podman manifest push mylist:v1.11 docker-daemon:registry.example.org/mylist:v1.11

  **oci-archive:**_path_**:**_tag_
  An image _tag_ in a directory compliant with "Open Container Image Layout Specification" at _path_.

    $ podman manifest push mylist:v1.11 oci-archive:/tmp/mylist

## EXAMPLE

```
podman manifest push mylist:v1.11 docker://registry.example.org/mylist:v1.11
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-manifest(1)](podman-manifest.1.md)**
