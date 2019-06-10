![PODMAN logo](logo/podman-logo-source.svg)

# podmanimage

## Overview

This directory contains the Dockerfiles necessary to create the three podmanimage container
images that are housed on quay.io under the podman account.  All three repositories where
the images live are public and can be pulled without credentials.  These container images are secured and the
resulting containers can run safely with privileges within the container.  The container images are built
using the latest Fedora and then Podman is installed into them:

  * quay.io/podman/stable - This image is built using the latest stable version of Podman in a Fedora based container.  Built with podman/stable/Dockerfile.
  * quay.io/podman/upstream - This image is built using the latest code found in this GitHub repository.  When someone creates a commit and pushes it, the image is created.  Due to that the image changes frequently and is not guaranteed to be stable.  Built with podmanimage/upstream/Dockerfile.
  * quay.io/podman/testing - This image is built using the latest version of Podman that is or was in updates testing for Fedora.  At times this may be the same as the stable image.  This container image will primarily be used by the development teams for verification testing when a new package is created.  Built with podmanimage/testing/Dockerfile.

## Sample Usage


```
podman pull docker://quay.io/podman/stable:latest

podman run --privileged stable podman version

# Create a directory on the host to mount the container's
# /var/lib/container directory to so containers can be
# run within the container.
mkdir /var/lib/mycontainer

# Run the image detached using the host's network in a container name
# podmanctr, turn off label and seccomp confinement in the container
# and then do a little shell hackery to keep the container up and running.
podman run --detach --name=podmanctr --net=host --security-opt label=disable --security-opt seccomp=unconfined --device /dev/fuse:rw -v /var/lib/mycontainer:/var/lib/containers:Z --privileged  stable sh -c 'while true ;do wait; done'

podman exec -it  podmanctr /bin/sh

# Now inside of the container

podman pull alpine

podman images

exit
```
