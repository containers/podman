#!/bin/bash

# N/B: This script is not intended to be run by humans.  It is used to configure the
# rhel base image for importing, so that it will boot in GCE

set -e

[[ "$1" == "post" ]] || exit 0  # pre stage is not needed

# Load in library (copied by packer, before this script was run)
source $GOSRC/$SCRIPT_BASE/lib.sh

req_env_var RHSM_COMMAND

install_ooe

rhsm_enable

echo "Setting up repos"
# Frequently needed
ooe.sh sudo yum -y install https://dl.fedoraproject.org/pub/epel/epel-release-latest-7.noarch.rpm

# Required for google to manage ssh keys
ooe.sh sudo tee /etc/yum.repos.d/google-cloud-sdk.repo << EOM
[google-cloud-compute]
name=google-cloud-compute
baseurl=https://packages.cloud.google.com/yum/repos/google-cloud-compute-el7-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
       https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOM

echo "Updating all packages"
ooe.sh sudo yum -y update

echo "Installing/removing packages"
ooe.sh sudo yum -y install rng-tools google-compute-engine google-compute-engine-oslogin

echo "Enabling critical services"
ooe.sh sudo systemctl enable \
    rngd \
    google-accounts-daemon \
    google-clock-skew-daemon \
    google-instance-setup \
    google-network-daemon \
    google-shutdown-scripts \
    google-startup-scripts

rhel_exit_handler  # release subscription!

echo "Configuring boot"
cat << "EOF" | sudo tee /etc/default/grub
GRUB_TIMEOUT=0
GRUB_DISTRIBUTOR="$(sed 's, release .*$,,g' /etc/system-release)"
GRUB_DEFAULT=saved
GRUB_DISABLE_SUBMENU=true
GRUB_TERMINAL="serial console"
GRUB_SERIAL_COMMAND="serial --speed=38400"
GRUB_CMDLINE_LINUX="crashkernel=auto console=ttyS0,38400n8"
GRUB_DISABLE_RECOVERY="true"
EOF
sudo grub2-mkconfig -o /boot/grub2/grub.cfg

echo "Configuring networking"
ooe.sh sudo nmcli connection modify 'System eth0' 802-3-ethernet.mtu 1460
ooe.sh sudo nmcli connection modify 'System eth0' connection.autoconnect yes
ooe.sh sudo nmcli connection modify 'System eth0' connection.autoconnect-priority
ooe.sh sudo nmcli connection modify 'System eth0' ipv4.method auto
ooe.sh sudo nmcli connection modify 'System eth0' ipv4.dhcp-send-hostname yes
ooe.sh sudo nmcli connection modify 'System eth0' ipv4.dhcp-timeout 0
ooe.sh sudo nmcli connection modify 'System eth0' ipv4.never-default no
ooe.sh /usr/bin/google_instance_setup

rh_finalize

echo "SUCCESS!"
