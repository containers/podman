FROM golang:1.8

# libseccomp in jessie is not _quite_ new enough -- need backports version
RUN echo 'deb http://httpredir.debian.org/debian jessie-backports main' > /etc/apt/sources.list.d/backports.list

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
    iptables \
    pkg-config \
    libaio-dev \
    libcap-dev \
    libfuse-dev \
    libostree-dev \
    libprotobuf-dev \
    libprotobuf-c0-dev \
    libseccomp2/jessie-backports \
    libseccomp-dev/jessie-backports \
    libtool \
    libudev-dev \
    protobuf-c-compiler \
    protobuf-compiler \
    python-minimal \
    libglib2.0-dev \
    libapparmor-dev \
    btrfs-tools \
    libdevmapper1.02.1 \
    libdevmapper-dev \
    libgpgme11-dev \
    liblzma-dev \
    netcat \
    socat \
    --no-install-recommends \
    && apt-get clean

# install bats
RUN cd /tmp \
    && git clone https://github.com/sstephenson/bats.git \
    && cd bats \
    && git reset --hard 03608115df2071fff4eaaff1605768c275e5f81f \
    && ./install.sh /usr/local

# install criu
ENV CRIU_VERSION 1.7
RUN mkdir -p /usr/src/criu \
    && curl -sSL https://github.com/xemul/criu/archive/v${CRIU_VERSION}.tar.gz | tar -v -C /usr/src/criu/ -xz --strip-components=1 \
    && cd /usr/src/criu \
    && make install-criu \
    && rm -rf /usr/src/criu

# Install runc
ENV RUNC_COMMIT 84a082bfef6f932de921437815355186db37aeb1
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
ENV CRIO_COMMIT 814c6ab0913d827543696b366048056a31d9529c
RUN set -x \
	&& export GOPATH="$(mktemp -d)" \
	&& git clone https://github.com/kubernetes-incubator/cri-o.git "$GOPATH/src/github.com/kubernetes-incubator/cri-o.git" \
	&& cd "$GOPATH/src/github.com/kubernetes-incubator/cri-o.git" \
	&& git fetch origin --tags \
	&& git checkout -q "$CRIO_COMMIT" \
	&& mkdir bin \
	&& make conmon \
	&& install -D -m 755 bin/conmon /usr/libexec/crio/conmon \
	&& rm -rf "$GOPATH"

# Install CNI plugins
ENV CNI_COMMIT 7480240de9749f9a0a5c8614b17f1f03e0c06ab9
RUN set -x \
       && export GOPATH="$(mktemp -d)" \
       && git clone https://github.com/containernetworking/plugins.git "$GOPATH/src/github.com/containernetworking/plugins" \
       && cd "$GOPATH/src/github.com/containernetworking/plugins" \
       && git checkout -q "$CNI_COMMIT" \
       && ./build.sh \
       && mkdir -p /usr/libexec/cni \
       && cp bin/* /usr/libexec/cni \
       && rm -rf "$GOPATH"

# Install crictl
ENV CRICTL_COMMIT 16e6fe4d7199c5689db4630a9330e6a8a12cecd1
RUN set -x \
       && export GOPATH="$(mktemp -d)" \
       && git clone https://github.com/kubernetes-incubator/cri-tools.git "$GOPATH/src/github.com/kubernetes-incubator/cri-tools" \
       && cd "$GOPATH/src/github.com/kubernetes-incubator/cri-tools" \
       && git checkout -q "$CRICTL_COMMIT" \
       && go install github.com/kubernetes-incubator/cri-tools/cmd/crictl \
       && cp "$GOPATH"/bin/crictl /usr/bin/ \
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

# Install cni config
#RUN make install.cni
RUN mkdir -p /etc/cni/net.d/
COPY cni/87-podman-bridge.conflist /etc/cni/net.d/87-podman-bridge.conflist

# Make sure we have some policy for pulling images
RUN mkdir -p /etc/containers && curl https://raw.githubusercontent.com/projectatomic/registries/master/registries.fedora -o /etc/containers/registries.conf

COPY test/policy.json /etc/containers/policy.json
COPY test/redhat_sigstore.yaml /etc/containers/registries.d/registry.access.redhat.com.yaml

WORKDIR /go/src/github.com/projectatomic/libpod

ADD . /go/src/github.com/projectatomic/libpod
