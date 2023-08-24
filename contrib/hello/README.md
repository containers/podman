![PODMAN logo](https://raw.githubusercontent.com/containers/common/main/logos/podman-logo-full-vert.png)

# Podman Hello World image

## Overview

This directory contains the Containerfile and bash script necessary to create the
"hello" podman image housed on quay.io under the Podman account in a public
repository.  The image is public and can be pulled without credentials.

Using this image is helpful to:

 * Prove that basic Podman operations are working on the host.
 * Shows that the image was pulled from the quay.io container registry.
 * Container creation was successfully accomplished. (`podman ps -a`)
 * The created container was able to stream output to your terminal.

## Directory Contents

The contents of this directory contain:
 * ./Containerfile
 * ./podman_hello_world.c

## Sample Usage

To simply run the image:

```
podman run quay.io/podman/hello

!... Hello Podman World ...!

         .--"--.
       / -     - \
      / (O)   (O) \
   ~~~| -=(,Y,)=- |
    .---. /`  \   |~~
 ~/  o  o \~~~~.----. ~~
  | =(X)= |~  / (O (O) \
   ~~~~~~~  ~| =(Y_)=-  |
  ~~~~    ~~~|   U      |~~

Project:   https://github.com/containers/podman
Website:   https://podman.io
Documents: https://docs.podman.io
Twitter:   @Podman_io
```
To build the image yourself, copy the files from this directory into
a local directory and issue these commands:

```
podman build -t myhello .
podman run myhello
```

## Potential Issues:

The image runs as a rootless user with the UID set to `1000`.
If the /etc/subuid and /etch/subgid values are not set appropriately to run as a
rootless user on the host, an error like this might be raised:

```
Copying blob acab339ca1e8 done
ERRO[0002] Error while applying layer: ApplyLayer exit status 1 stdout:  stderr: potentially insufficient UIDs or GIDs available in user namespace (requested 0:12 for /var/spool/mail): Check /etc/subuid and /etc/subgid: lchown /var/spool/mail: invalid argument
Error: writing blob: adding layer with blob "sha256:ee0cde9de8a68f171a8c03b0e9954abf18576947e2f3187e84d8c31ccd8f6a09": ApplyLayer exit status 1 stdout:  stderr: potentially insufficient UIDs or GIDs available in user namespace (requested 0:12 for /var/spool/mail): Check /etc/subuid and /etc/subgid: lchown /var/spool/mail: invalid argument
```

Please refer to this [blog post](https://www.redhat.com/sysadmin/rootless-podman) for further configuration information.

## THANKS!

Many Thanks to @afbjorklund for a great discussion during the
first revision of this container image that resulted in moving
from using bash to using C, and the ensuing changes to the
Containerfile.

Also many thanks to @mairin for the awesome ASCII art!
