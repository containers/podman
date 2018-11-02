#!/bin/bash

# N/B: This script is not intended to be run by humans.  It is used to configure the
# rhel base image for importing, so that it will boot in GCE

ooe.sh sudo yum -y erase "rh-amazon-rhui-client*"

# Frequently needed
ooe.sh sudo yum -y install https://dl.fedoraproject.org/pub/epel/epel-release-latest-7.noarch.rpm

# Required for google to manage ssh keys
sudo tee -a /etc/yum.repos.d/google-cloud-sdk.repo << EOM
[google-cloud-compute]
name=google-cloud-compute
baseurl=https://packages.cloud.google.com/yum/repos/google-cloud-compute-el7-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
       https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOM

ooe.sh sudo yum -y install google-compute-engine google-compute-engine-oslogin

echo "SUCCESS!"
