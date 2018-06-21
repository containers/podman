FROM golang:1.10

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
    go-md2man \
    iptables \
    pkg-config \
    libaio-dev \
    libcap-dev \
    libfuse-dev \
    libostree-dev \
    libprotobuf-dev \
    libprotobuf-c0-dev \
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
    python3-pip \
    python3-dateutil \
    python3-setuptools \
    --no-install-recommends \
    && apt-get clean

ADD . /go/src/github.com/projectatomic/libpod

RUN set -x && cd /go/src/github.com/projectatomic/libpod && make install.libseccomp.sudo

# Install runc
ENV RUNC_COMMIT ad0f5255060d36872be04de22f8731f38ef2d7b1
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
ENV CRIO_COMMIT 66788a10e57f42faf741c2f149d0ee6635063014
RUN set -x \
	&& export GOPATH="$(mktemp -d)" \
	&& git clone https://github.com/kubernetes-incubator/cri-o.git "$GOPATH/src/github.com/kubernetes-incubator/cri-o.git" \
	&& cd "$GOPATH/src/github.com/kubernetes-incubator/cri-o.git" \
	&& git fetch origin --tags \
	&& git checkout -q "$CRIO_COMMIT" \
	&& make \
	&& install -D -m 755 bin/conmon /usr/libexec/podman/conmon \
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

# Install buildah
RUN set -x \
       && export GOPATH=/go \
       && git clone https://github.com/projectatomic/buildah "$GOPATH/src/github.com/projectatomic/buildah" \
       && cd "$GOPATH/src/github.com/projectatomic/buildah" \
       && make \
       && make install

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

# Install python3 varlink module from pypi
RUN pip3 install varlink

COPY test/policy.json /etc/containers/policy.json
COPY test/redhat_sigstore.yaml /etc/containers/registries.d/registry.access.redhat.com.yaml

WORKDIR /go/src/github.com/projectatomic/libpod
