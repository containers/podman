FROM fedora:28
RUN dnf -y install \
      atomic-registries \
      btrfs-progs-devel \
      buildah \
      bzip2 \
      conmon \
      container-selinux \
      containernetworking-cni \
      containernetworking-cni-devel \
      device-mapper-devel \
      findutils \
      git \
      glib2-devel \
      glibc-static \
      gnupg \
      golang \
      gpgme-devel \
      iptables \
      libassuan-devel \
      libseccomp-devel \
      libselinux-devel \
      lsof \
      make \
      nmap-ncat \
      ostree-devel \
      procps-ng \
      python \
      python3-dateutil \
      python3-psutil \
      python3-pytoml \
      python3-varlink \
      skopeo-containers \
      slirp4netns \
      rsync \
      which \
      xz \
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
    go get -u github.com/mailru/easyjson/... && \
    install -D -m 755 "$GOPATH"/bin/easyjson /usr/bin/ && \
    make install.tools && \
    install -D -m 755 $GOSRC/contrib/gate/entrypoint.sh /usr/local/bin/ && \
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
