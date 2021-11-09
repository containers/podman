% podman-secret-create(1)

## NAME
podman\-secret\-create - Create a new secret

## SYNOPSIS
**podman secret create** [*options*] *name* *file|-*

## DESCRIPTION

Creates a secret using standard input or from a file for the secret content.

Create accepts a path to a file, or `-`, which tells podman to read the secret from stdin

A secret is a blob of sensitive data which a container needs at runtime but
should not be stored in the image or in source control, such as usernames and passwords,
TLS certificates and keys, SSH keys or other important generic strings or binary content (up to 500 kb in size).

Secrets will not be committed to an image with `podman commit`, and will not be in the archive created by a `podman export`

## OPTIONS

#### **--env**=*false*

Read secret data from environment variable

#### **--driver**=*driver*

Specify the secret driver (default **file**, which is unencrypted).

#### **--driver-opts**=*key1=val1,key2=val2*

Specify driver specific options

#### **--help**

Print usage statement.

## EXAMPLES

```
$ podman secret create my_secret ./secret.json
$ podman secret create --driver=file my_secret ./secret.json
$ printf <secret> | podman secret create my_secret -
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-secret(1)](podman-secret.1.md)**

## HISTORY
January 2021, Originally compiled by Ashley Cui <acui@redhat.com>
