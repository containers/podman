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
FROM registry.fedoraproject.org/fedora:latest

# Don't include container-selinux and remove
# directories used by yum that are just taking
# up space.
RUN yum -y update; rpm --restore shadow-utils 2>/dev/null; yum -y install podman fuse-overlayfs --exclude container-selinux --enablerepo updates-testing; rm -rf /var/cache /var/log/dnf* /var/log/yum.*

RUN useradd podman; \
echo podman:10000:5000 > /etc/subuid; \
echo podman:10000:5000 > /etc/subgid;

ADD https://raw.githubusercontent.com/containers/podman/main/contrib/podmanimage/stable/containers.conf /etc/containers/containers.conf
ADD https://raw.githubusercontent.com/containers/podman/main/contrib/podmanimage/stable/podman-containers.conf /home/podman/.config/containers/containers.conf

RUN mkdir -p /home/podman/.local/share/containers; chown podman:podman -R /home/podman

# Note VOLUME options must always happen after the chown call above
# RUN commands can not modify existing volumes
VOLUME /var/lib/containers
VOLUME /home/podman/.local/share/containers

# chmod containers.conf and adjust storage.conf to enable Fuse storage.
RUN chmod 644 /etc/containers/containers.conf; sed -i -e 's|^#mount_program|mount_program|g' -e '/additionalimage.*/a "/var/lib/shared",' -e 's|^mountopt[[:space:]]*=.*$|mountopt = "nodev,fsync=0"|g' /etc/containers/storage.conf
RUN mkdir -p /var/lib/shared/overlay-images /var/lib/shared/overlay-layers /var/lib/shared/vfs-images /var/lib/shared/vfs-layers; touch /var/lib/shared/overlay-images/images.lock; touch /var/lib/shared/overlay-layers/layers.lock; touch /var/lib/shared/vfs-images/images.lock; touch /var/lib/shared/vfs-layers/layers.lock

ENV _CONTAINERS_USERNS_CONFIGURED=""
