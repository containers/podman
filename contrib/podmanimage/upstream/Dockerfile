# git/Dockerfile
#
# Build a Podman container image from the latest
# upstream version of Podman on GitHub.
# https://github.com/containers/libpod
# This image can be used to create a secured container
# that runs safely with privileges within the container.
# The containers created by this image also come with a
# Podman development environment in /root/podman.
#
FROM fedora:latest
ENV GOPATH=/root/podman

# Install the software required to build Podman.
# Then create a directory and clone from the Podman
# GitHub repository, make and install Podman
# to the container.
# Finally remove the podman directory and a few other packages
# that are needed for building but not running Podman
RUN useradd build; yum -y update; yum -y reinstall shadow-utils; yum -y install --exclude container-selinux \
     --enablerepo=updates-testing \
     atomic-registries \
     btrfs-progs-devel \
     containernetworking-cni \
     device-mapper-devel \
     git \
     glib2-devel \
     glibc-devel \
     glibc-static \
     go \
     golang-github-cpuguy83-go-md2man \
     gpgme-devel \
     iptables \
     libassuan-devel \
     libgpg-error-devel \
     libseccomp-devel \
     libselinux-devel \
     make \
     pkgconfig \
     runc \
     fuse-overlayfs \
     fuse3 \
     containers-common; \
     mkdir /root/podman; \
     git clone https://github.com/containers/libpod /root/podman/src/github.com/containers/libpod; \
     cd /root/podman/src/github.com/containers/libpod; \
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
     cp $GOPATH/src/github.com/containers/libpod/libpod.conf /usr/share/containers; \
     # Adjust libpod.conf to write logging to a file
     sed -i 's/# events_logger = "journald"/events_logger = "file"/g' /usr/share/containers/libpod.conf; \
     rm -rf /root/podman/*; \
     yum -y remove git golang go-md2man make; \
     yum clean all;

# Adjust storage.conf to enable Fuse storage.
RUN sed -i -e 's|^#mount_program|mount_program|g' -e '/additionalimage.*/a "/var/lib/shared",' /etc/containers/storage.conf
RUN mkdir -p /var/lib/shared/overlay-images /var/lib/shared/overlay-layers; touch /var/lib/shared/overlay-images/images.lock; touch /var/lib/shared/overlay-layers/layers.lock

# Set up environment variables to note that this is
# not starting with usernamespace and default to
# isolate the filesystem with chroot.
ENV _BUILDAH_STARTED_IN_USERNS="" BUILDAH_ISOLATION=chroot
