% podman-secret-create 1

## NAME
podman\-secret\-create - Create a new secret

## SYNOPSIS
**podman secret create** [*options*] *name* *file|-*

## DESCRIPTION

Creates a secret using standard input or from a file for the secret content.

Create accepts a path to a file, or `-`, which tells podman to read the secret from stdin

A secret is a blob of sensitive data which a container needs at runtime but
is not stored in the image or in source control, such as usernames and passwords,
TLS certificates and keys, SSH keys or other important generic strings or binary content (up to 500 kb in size).

Secrets are not committed to an image with `podman commit`, and does not get committed in the archive created by a `podman export` command.

## OPTIONS

#### **--driver**, **-d**=*driver*

Specify the secret driver (default **file**, which is unencrypted).

#### **--driver-opts**=*key1=val1,key2=val2*

Specify driver specific options.

#### **--env**=*false*

Read secret data from environment variable.

#### **--help**

Print usage statement.

#### **--label**, **-l**=*key=val1,key2=val2*

Add label to secret. These labels can be viewed in podman secrete inspect or ls.

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
