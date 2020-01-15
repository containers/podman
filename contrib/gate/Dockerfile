FROM fedora:31
RUN dnf -y install \
      btrfs-progs-devel \
      bzip2 \
      container-selinux \
      containernetworking-cni \
      device-mapper-devel \
      findutils \
      git \
      glib2-devel \
      glibc-static \
      golang \
      gpgme-devel \
      iptables \
      libassuan-devel \
      libseccomp-devel \
      libselinux-devel \
      lsof \
      make \
      nmap-ncat \
      procps-ng \
      python \
      python3-dateutil \
      python3-psutil \
      python3-pytoml \
      python3-pyyaml \
      python3-varlink \
      rsync \
      slirp4netns \
      unzip \
      which \
      xz \
      zip \
      && dnf clean all

ENV GOPATH="/go" \
    PATH="/go/bin:/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin" \
    SRCPATH="/usr/src/libpod" \
    GOSRC="/go/src/github.com/containers/libpod"

# Only needed for installing build-time dependencies
COPY / $GOSRC

WORKDIR $GOSRC

# Install dependencies
RUN set -x && \
    install -D -m 755 $GOSRC/contrib/gate/entrypoint.sh /usr/local/bin/ && \
    python3 -m pip install pre-commit && \
    rm -rf "$GOSRC"

# Install cni config
#RUN make install.cni
RUN mkdir -p /etc/cni/net.d/
COPY cni/87-podman-bridge.conflist /etc/cni/net.d/87-podman-bridge.conflist

# Make sure we have some policy for pulling images
RUN mkdir -p /etc/containers
COPY test/policy.json /etc/containers/policy.json
COPY test/redhat_sigstore.yaml /etc/containers/registries.d/registry.access.redhat.com.yaml

VOLUME ["/usr/src/libpod"]
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
