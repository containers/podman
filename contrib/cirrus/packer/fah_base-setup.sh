
# N/B: This script is not intended to be run by humans.  It is used to configure the
# FAH base image for importing, so that it will boot in GCE.

set -e

# Load in library (copied by packer, before this script was run)
source $GOSRC/$SCRIPT_BASE/lib.sh

install_ooe

if [[ "$1" == "pre" ]]
then
    echo "Upgrading Atomic Host"
    setenforce 0
    ooe.sh atomic host upgrade

    echo "Configuring Repositories"
    ooe.sh sudo tee /etc/yum.repos.d/ngompa-gce-oslogin.repo <<EOF
[ngompa-gce-oslogin]
name=Copr repo for gce-oslogin owned by ngompa
baseurl=https://copr-be.cloud.fedoraproject.org/results/ngompa/gce-oslogin/fedora-\$releasever-\$basearch/
type=rpm-md
skip_if_unavailable=True
gpgcheck=1
gpgkey=https://copr-be.cloud.fedoraproject.org/results/ngompa/gce-oslogin/pubkey.gpg
repo_gpgcheck=0
enabled=1
enabled_metadata=1
EOF
    echo "Installing necessary packages and  google services"
    # Google services are enabled by default, upon install.
    ooe.sh rpm-ostree install rng-tools google-compute-engine google-compute-engine-oslogin
    echo "Rebooting..."
    systemctl reboot  # Required for upgrade + package installs to be active
elif [[ "$1" == "post" ]]
then
    echo "Enabling necessary services"
    systemctl enable rngd  # Must reboot before enabling
    rh_finalize
    echo "SUCCESS!"
else
    echo "Expected to be called with 'pre' or 'post'"
    exit 6
fi
