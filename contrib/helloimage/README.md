![PODMAN logo](../../logo/podman-logo-source.svg)

# Podman Hello World image

## Overview

This directory contains the Containerfile and bash script necessary to create the
"hello" podman image housed on quay.io under the Podman account in a public
repository.  The image is public and can be pulled without credentials.

Using this image is helpful to:

 * Prove that basic Podman operations are working on the host.
 * Shows that the image was pulled from the quay.io container registry.
 * Container creation was successfuly accomplished. (`podman ps -a`)
 * The created container was able to stream output to your terminal.

## Directory Contents

The contents of this directory contain:
 * ./Containerfile
 * ./podman_hello_world.bash

## Sample Usage

To simply run the image:

```
podman run quay.io/podman/hello

! ... Hello Podman World ...!

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
chmod 755 ./podman_hello_world.bash
podman build -t myhello .
podman run myhello
```
