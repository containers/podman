[comment]: <> (***ATTENTION*** ***WARNING*** ***ALERT*** ***CAUTION*** ***DANGER***)
[comment]: <> ()
[comment]: <> (ANY changes made to this file, once committed/merged must)
[comment]: <> (be manually copy/pasted -in markdown- into the description)
[comment]: <> (field on Quay at the following locations:)
[comment]: <> ()
[comment]: <> (https://quay.io/repository/containers/podman)
[comment]: <> (https://quay.io/repository/podman/stable)
[comment]: <> (https://quay.io/repository/podman/testing)
[comment]: <> (https://quay.io/repository/podman/upstream)
[comment]: <> ()
[comment]: <> (***ATTENTION*** ***WARNING*** ***ALERT*** ***CAUTION*** ***DANGER***)

![PODMAN logo](https://raw.githubusercontent.com/containers/common/main/logos/podman-logo-full-vert.png)

# podmanimage

## Overview

This directory contains the Containerfiles necessary to create the podmanimage container
images that are housed on quay.io under the Podman account.  All repositories where
the images live are public and can be pulled without credentials.  These container images are secured and the
resulting containers can run safely with privileges within the container.

The container images are built using the latest Fedora and then Podman is installed into them.
The PATH in the container images is set to the default PATH provided by Fedora.  Also, the
ENTRYPOINT and the WORKDIR variables are not set within these container images, as such they
default to `/`.

The container images are:

  * `quay.io/containers/podman:<version>` and `quay.io/podman/stable:<version>` -
    These images are built daily.  They are intended to contain an unchanging
    and stable version of podman. For the most recent `<version>` tags (`vX`,
    `vX.Y`, and `vX.Y.Z`) the image contents will be updated daily to incorporate
    (especially) security upgrades.  For build details, please [see the
    configuration file](stable/Containerfile).
  * `quay.io/containers/podman:latest` and `quay.io/podman/stable:latest` -
    Built daily using the same Containerfile as above.  The Podman version
    will remain the "latest" available in Fedora, however the other image
    contents may vary compared to the version-tagged images.
  * `quay.io/podman/testing:latest` - This image is built daily, using the
    latest version of Podman that was in the Fedora `updates-testing` repository.
    The image is Built with [the testing Containerfile](testing/Containerfile).
  * `quay.io/podman/upstream:latest` - This image is built daily using the latest
    code found in this GitHub repository.  Due to the image changing frequently,
    it's not guaranteed to be stable or even executable.  The image is built with
    [the upstream Containerfile](upstream/Containerfile). Note the actual compilation
    of upstream podman [occurs continuously in
    COPR](https://copr.fedorainfracloud.org/coprs/rhcontainerbot/podman-next/).

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
podman run --detach --name=podmanctr --net=host --security-opt label=disable --security-opt seccomp=unconfined --device /dev/fuse:rw -v /var/lib/mycontainer:/var/lib/containers:Z --privileged  stable sh -c 'while true ;do sleep 100000 ; done'

podman exec -it  podmanctr /bin/sh

# Now inside of the container

podman pull alpine

podman images

exit
```

**Note:** If you encounter a `fuse: device not found` error when running the container image, it is likely that
the fuse kernel module has not been loaded on your host system.  Use the command `modprobe fuse` to load the
module and then run the container image.  To enable this automatically at boot time, you can add a configuration
file to `/etc/modules.load.d`.  See `man modules-load.d` for more details.

### Blog Post with Details

Dan Walsh wrote a blog post on the [Enable Sysadmin](https://www.redhat.com/sysadmin/) site titled [How to use Podman inside of a container](https://www.redhat.com/sysadmin/podman-inside-container).  In it, he details how to use these images as a rootful and as a rootless user.  Please refer to this blog for more detailed information.
