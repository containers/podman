#!/bin/bash

TMPDIR=$(mktmp -d)
trap "rm -rf $TMPDIR" EXIT

cd $TMPDIR
cat <<EOF>google-cloud.repo
[google-cloud-compute]
name=Google Cloud Compute
baseurl=https://packages.cloud.google.com/yum/repos/google-cloud-compute-el7-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
       https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOF

cat <<EOF>gcegizmo.dockerfile
FROM registry.centos.org/centos:latest
MAINTAINER Chris Evich <cevich@redhat.com>
ENV container podman
ADD /google-cloud.repo /etc/yum.repos.d/
RUN yum update -y && \
    yum install -y epel-release && \
    yum install -y \
        python-google-compute-engine \
        google-compute-engine-oslogin \
        google-compute-engine && \
    yum clean all
RUN (cd /lib/systemd/system/sysinit.target.wants/; for i in *; do [ $i == systemd-tmpfiles-setup.service ] || rm -f $i; done); \
rm -f /lib/systemd/system/multi-user.target.wants/*;\
rm -f /etc/systemd/system/*.wants/*;\
rm -f /lib/systemd/system/local-fs.target.wants/*; \
rm -f /lib/systemd/system/sockets.target.wants/*udev*; \
rm -f /lib/systemd/system/sockets.target.wants/*initctl*; \
rm -f /lib/systemd/system/basic.target.wants/*;\
rm -f /lib/systemd/system/anaconda.target.wants/*;
VOLUME [ "/sys/fs/cgroup" ]
ENTRYPOINT ["/usr/sbin/init"]
EOF

podman build -t gcegizmo --force-rm --pull .

# drop systemd service unit to start gce-doo-dads container on boot

# drop dummy 'git' command on host
