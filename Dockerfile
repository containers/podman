FROM golang:1.12

RUN apt-get update && apt-get install -y \
    apparmor \
    autoconf \
    automake \
    bison \
    build-essential \
    curl \
    e2fslibs-dev \
    gawk \
    gettext \
    go-md2man \
    iptables \
    pkg-config \
    libaio-dev \
    libcap-dev \
    libfuse-dev \
    libnet-dev \
    libnl-3-dev \
    libostree-dev \
    libprotobuf-dev \
    libprotobuf-c-dev \
    libseccomp2 \
    libseccomp-dev \
    libtool \
    libudev-dev \
    protobuf-c-compiler \
    protobuf-compiler \
    libglib2.0-dev \
    libapparmor-dev \
    btrfs-tools \
    libdevmapper1.02.1 \
    libdevmapper-dev \
    libgpgme11-dev \
    liblzma-dev \
    netcat \
    socat \
    lsof \
    xz-utils \
    unzip \
    python3-yaml \
    --no-install-recommends \
    && apt-get clean

# Install runc
ENV RUNC_COMMIT 029124da7af7360afa781a0234d1b083550f797c
RUN set -x \
	&& export GOPATH="$(mktemp -d)" \
	&& git clone https://github.com/opencontainers/runc.git "$GOPATH/src/github.com/opencontainers/runc" \
	&& cd "$GOPATH/src/github.com/opencontainers/runc" \
	&& git fetch origin --tags \
	&& git checkout -q "$RUNC_COMMIT" \
	&& make static BUILDTAGS="seccomp selinux" \
	&& cp runc /usr/bin/runc \
	&& rm -rf "$GOPATH"

# Install conmon
ENV CONMON_COMMIT 6f3572558b97bc60dd8f8c7f0807748e6ce2c440
RUN set -x \
	&& export GOPATH="$(mktemp -d)" \
	&& git clone https://github.com/containers/conmon.git "$GOPATH/src/github.com/containers/conmon.git" \
	&& cd "$GOPATH/src/github.com/containers/conmon.git" \
	&& git fetch origin --tags \
	&& git checkout -q "$CONMON_COMMIT" \
	&& make \
	&& install -D -m 755 bin/conmon /usr/libexec/podman/conmon \
	&& rm -rf "$GOPATH"

# Install CNI plugins
ENV CNI_COMMIT 485be65581341430f9106a194a98f0f2412245fb
RUN set -x \
       && export GOPATH="$(mktemp -d)" \
       && git clone https://github.com/containernetworking/plugins.git "$GOPATH/src/github.com/containernetworking/plugins" \
       && cd "$GOPATH/src/github.com/containernetworking/plugins" \
       && git checkout -q "$CNI_COMMIT" \
       && ./build.sh \
       && mkdir -p /usr/libexec/cni \
       && cp bin/* /usr/libexec/cni \
       && rm -rf "$GOPATH"

# Install ginkgo
RUN set -x \
       && export GOPATH=/go \
       && go get -u github.com/onsi/ginkgo/ginkgo \
       && install -D -m 755 "$GOPATH"/bin/ginkgo /usr/bin/

# Install gomega
RUN set -x \
       && export GOPATH=/go \
       && go get github.com/onsi/gomega/...

# Install latest stable criu version
RUN set -x \
      && cd /tmp \
      && git clone https://github.com/checkpoint-restore/criu.git \
      && cd criu \
      && make \
      && install -D -m 755  criu/criu /usr/sbin/ \
      && rm -rf /tmp/criu

# Install cni config
#RUN make install.cni
RUN mkdir -p /etc/cni/net.d/
COPY cni/87-podman-bridge.conflist /etc/cni/net.d/87-podman-bridge.conflist

# Make sure we have some policy for pulling images
RUN mkdir -p /etc/containers && curl https://raw.githubusercontent.com/projectatomic/registries/master/registries.fedora -o /etc/containers/registries.conf

COPY test/policy.json /etc/containers/policy.json
COPY test/redhat_sigstore.yaml /etc/containers/registries.d/registry.access.redhat.com.yaml

ADD . /go/src/github.com/containers/libpod

RUN set -x && cd /go/src/github.com/containers/libpod

WORKDIR /go/src/github.com/containers/libpod
