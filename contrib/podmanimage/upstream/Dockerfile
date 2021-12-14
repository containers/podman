# git/Dockerfile
#
# Build a Podman container image from the latest
# upstream version of Podman on GitHub.
# https://github.com/containers/podman
# This image can be used to create a secured container
# that runs safely with privileges within the container.
# The containers created by this image also come with a
# Podman development environment in /root/podman.
#
FROM registry.fedoraproject.org/fedora:latest
ENV GOPATH=/root/podman

# Install the software required to build Podman.
# Then create a directory and clone from the Podman
# GitHub repository, make and install Podman
# to the container.
# Finally remove the podman directory and a few other packages
# that are needed for building but not running Podman
RUN yum -y update; rpm --restore shadow-utils 2>/dev/null;  yum -y install --exclude container-selinux \
     --enablerepo=updates-testing \
     btrfs-progs-devel \
     containernetworking-cni \
     conmon \
     device-mapper-devel \
     git \
     glib2-devel \
     glibc-devel \
     glibc-static \
     go \
     golang-github-cpuguy83-md2man \
     gpgme-devel \
     iptables \
     libassuan-devel \
     libgpg-error-devel \
     libseccomp-devel \
     libselinux-devel \
     make \
     pkgconfig \
     crun \
     fuse-overlayfs \
     fuse3 \
     containers-common \
     podman-plugins; \
     mkdir /root/podman; \
     git clone https://github.com/containers/podman /root/podman/src/github.com/containers/podman; \
     cd /root/podman/src/github.com/containers/podman; \
     make BUILDTAGS="selinux seccomp"; \
     make install PREFIX=/usr; \
     cd /root/podman; \
     git clone https://github.com/containers/conmon /root/podman/conmon; \
     cd conmon; \
     make; \
     install -D -m 755 bin/conmon /usr/libexec/podman/conmon; \
     git clone https://github.com/containernetworking/plugins.git $GOPATH/src/github.com/containernetworking/plugins; \
     cd $GOPATH/src/github.com/containernetworking/plugins; \
     ./build_linux.sh; \
     mkdir -p /usr/libexec/cni; \
     \cp -fR bin/* /usr/libexec/cni; \
     mkdir -p /etc/cni/net.d; \
     curl -qsSL https://raw.githubusercontent.com/containers/libpod/master/cni/87-podman-bridge.conflist | tee /etc/cni/net.d/99-loopback.conf; \
     mkdir -p /usr/share/containers; \
     rm -rf /root/podman/*; \
     yum -y remove git golang go-md2man make; \
     yum clean all;

RUN useradd podman; \
echo podman:10000:5000 > /etc/subuid; \
echo podman:10000:5000 > /etc/subgid;

ADD https://raw.githubusercontent.com/containers/libpod/master/contrib/podmanimage/stable/containers.conf /etc/containers/containers.conf
ADD https://raw.githubusercontent.com/containers/libpod/master/contrib/podmanimage/stable/podman-containers.conf /home/podman/.config/containers/containers.conf

RUN mkdir -p /home/podman/.local/share/containers; chown podman:podman -R /home/podman

# Note VOLUME options must always happen after the chown call above
# RUN commands can not modify existing volumes
VOLUME /var/lib/containers
VOLUME /home/podman/.local/share/containers

# chmod containers.conf and adjust storage.conf to enable Fuse storage.
RUN chmod 644 /etc/containers/containers.conf; sed -i -e 's|^#mount_program|mount_program|g' -e '/additionalimage.*/a "/var/lib/shared",' -e 's|^mountopt[[:space:]]*=.*$|mountopt = "nodev,fsync=0"|g' /etc/containers/storage.conf
RUN mkdir -p /var/lib/shared/overlay-images /var/lib/shared/overlay-layers /var/lib/shared/vfs-images /var/lib/shared/vfs-layers; touch /var/lib/shared/overlay-images/images.lock; touch /var/lib/shared/overlay-layers/layers.lock; touch /var/lib/shared/vfs-images/images.lock; touch /var/lib/shared/vfs-layers/layers.lock

ENV _CONTAINERS_USERNS_CONFIGURED=""
