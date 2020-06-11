FROM registry.fedoraproject.org/fedora:latest

# This container image is utilized by the containers CI automation system
# for building and testing libpod inside a container environment.
# It is assumed that the source to be tested will overwrite $GOSRC (below)
# at runtime.
ENV GOPATH=/var/tmp/go
ENV GOSRC=$GOPATH/src/github.com/containers/libpod
ENV SCRIPT_BASE=./contrib/cirrus
ENV PACKER_BASE=$SCRIPT_BASE/packer

# Only add minimal tooling necessary to complete setup.
ADD / $GOSRC
WORKDIR $GOSRC

# Re-use repositories and package setup as in VMs under CI
RUN bash $PACKER_BASE/fedora_packaging.sh && \
    dnf clean all && \
    rm -rf /var/cache/dnf

# Mirror steps taken under CI
RUN bash -c 'source $GOSRC/$SCRIPT_BASE/lib.sh && install_test_configs'
