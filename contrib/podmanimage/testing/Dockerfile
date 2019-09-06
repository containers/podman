# testing/Dockerfile
#
# Build a Podman image using the latest
# version of Podman that is in updates-testing
# on the Fedoras Updates System.  At times this
# may be the same the latest stable version.
# https://bodhi.fedoraproject.org/updates/?search=podman
# This image can be used to create a secured container
# that runs safely with privileges within the container.
#
FROM fedora:latest

# Don't include container-selinux and remove
# directories used by dnf that are just taking
# up space.
RUN yum -y install podman fuse-overlayfs --exclude container-selinux --enablerepo updates-testing; rm -rf /var/cache /var/log/dnf* /var/log/yum.*

# Adjust storage.conf to enable Fuse storage.
RUN sed -i -e 's|^#mount_program|mount_program|g' -e '/additionalimage.*/a "/var/lib/shared",' /etc/containers/storage.conf
RUN mkdir -p /var/lib/shared/overlay-images /var/lib/shared/overlay-layers; touch /var/lib/shared/overlay-images/images.lock; touch /var/lib/shared/overlay-layers/layers.lock

# Adjust libpod.conf to write logging to a file
RUN sed -i 's/# events_logger = "journald"/events_logger = "file"/g' /usr/share/containers/libpod.conf

# Set up environment variables to note that this is
# not starting with usernamespace and default to
# isolate the filesystem with chroot.
ENV _BUILDAH_STARTED_IN_USERNS="" BUILDAH_ISOLATION=chroot
