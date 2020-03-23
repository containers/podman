FROM fedora:29 AS build

COPY --from=registry.svc.ci.openshift.org/openshift/origin-v4.4:machine-os-content /srv/ /srv/
RUN set -x && yum install -y ostree yum-utils selinux-policy-targeted && \
    commit=$( find /srv -name *.commit | sed -Ee 's|.*objects/(.+)/(.+)\.commit|\1\2|' | head -1 ) && \
    mkdir /tmp/working && cd /tmp/working && \
    yumdownloader --disablerepo=* --enablerepo=built --destdir=/tmp/rpms podman && \
    ls /tmp/rpms/ && (cd /tmp/rpms/ && ls podman*) && \
    for i in $(find /tmp/rpms/ -name podman* -iname *.rpm); do echo "Extracting $i ..."; rpm2cpio $i | cpio -div; done && \
    if [[ -d etc ]]; then mv etc usr/; fi && \
    mkdir -p /tmp/tmprootfs/etc && \
    ostree --repo=/srv/repo checkout -U $commit --subpath /usr/etc/selinux /tmp/tmprootfs/etc/selinux && \
    ostree --repo=/srv/repo commit --parent=$commit --tree=ref=$commit --tree=dir=. \
        --selinux-policy /tmp/tmprootfs \
        -s "podman-ci-dev overlay RPMs" --branch=podman-ci-dev

FROM scratch
COPY --from=build /srv/ /srv/