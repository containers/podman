#!/bin/bash

# This script is called by packer on a vanilla CentOS VM, to setup the image
# used for building base images into images ready for CI testing.
# It's not intended to be used outside of this context.

set -e

# Load in library (copied by packer, before this script was run)
source $GOSRC/$SCRIPT_BASE/lib.sh

req_env_var "
    TIMESTAMP $TIMESTAMP
    GOSRC $GOSRC
    XFERBUCKET $XFERBUCKET
    SCRIPT_BASE $SCRIPT_BASE
    PACKER_BASE $PACKER_BASE
    GCP_PROJECT_ID $GCP_PROJECT_ID
    GOOGLE_APPLICATION_CREDENTIALS $GOOGLE_APPLICATION_CREDENTIALS
    FEDORA_IMAGE_URL $FEDORA_IMAGE_URL
    FEDORA_CSUM_URL $FEDORA_CSUM_URL
    FAH_IMAGE_URL $FAH_IMAGE_URL
    FAH_CSUM_URL $FAH_CSUM_URL
    RHEL_IMAGE_BASENAME $RHEL_IMAGE_BASENAME
    RHEL_IMAGE_FILE $RHEL_IMAGE_FILE
    RHEL_CSUM_FILE $RHEL_CSUM_FILE
    REMOTE_IMAGE_NAME_FILE $REMOTE_IMAGE_NAME_FILE
"

install_ooe

echo "Updating packages"
ooe.sh sudo yum -y update

echo "Configuring repositories"
ooe.sh sudo yum -y install centos-release-scl epel-release

echo "Installing packages"
ooe.sh sudo yum -y install \
    google-cloud-sdk \
    qemu-kvm \
    make

# When building more than one base-image, if there is a failure
# ensure no potentially partially built images remain
CLEAN_IMAGES=""
clean_images() {
    RET=$?
    if [[ "$RET" -ne "0" ]]
    then
        echo "There was an error, cleaning up images:"
        echo "$CLEAN_IMAGES"
        set +e
        echo "$CLEAN_IMAGES" | while read IMGNAME
        do
            [[ -z "$IMGNAME" ]] || gcloud compute images --quiet delete $IMGNAME
        done
    fi
}
trap clean_images EXIT

IMAGE_PREFIX=""
IMAGE_URL=""
CSUM_URL=""
SCRIPT=""
base_image_script(){
    req_env_var "
        GOSRC $GOSRC
        TIMESTAMP $TIMESTAMP
        XFERBUCKET $XFERBUCKET
        IMAGE_PREFIX $IMAGE_PREFIX
        IMAGE_URL $IMAGE_URL
        CSUM_URL $CSUM_URL
        SCRIPT $SCRIPT
    "
    echo "Downloading, modifying, and importing base image $IMAGE_PREFIX"
    # Assume $GOSRC directory will be cleaned up after script runs
    cd $GOSRC
    IMAGE_BASENAME="$(basename $IMAGE_URL)"
    if [[ ! -r "$IMAGE_BASENAME" ]]
    then
        ooe.sh curl -O "$IMAGE_URL"
        curl "$CSUM_URL" | grep "$IMAGE_BASENAME" | \
            sha256sum --quiet --status --warn --check -
    fi
    xzcat "$IMAGE_BASENAME" > disk.raw  # name required by GCE
    sudo kpartx -a disk.raw
    LOOPDEV=$(sudo losetup --associated disk.raw --output name | tail -n +2)
    LOOPPART="/dev/mapper/$(basename $LOOPDEV)p1"
    sudo mount -o loop,rw $LOOPPART /mnt
    # This doesn't exist on atomic-host image
    [[ ! -r "/mnt/etc/resolv.conf" ]] || \
        sudo cp /etc/resolv.conf /mnt/etc/  # required for networking to function
    echo "Executing $SCRIPT inside image chroot"
    cat "$SCRIPT" | ooe.sh sudo chroot /mnt bash
    echo -n "Finalizing image modifications"
    [[ ! -r "/mnt/etc/resolv.conf" ]] || \
        sudo truncate --size=0 /mnt/etc/resolv.conf  # would mess with VM after boot
    sudo umount /mnt
    sudo kpartx -d $LOOPDEV
    sudo losetup -d $LOOPDEV
    IMGNAME="${IMAGE_PREFIX}-${TIMESTAMP}"
    echo "Uploading ${IMGNAME}.tar.gz to bucket $XFERBUCKET"
    tar -Sczf ${IMGNAME}.tar.gz disk.raw
    ooe.sh gsutil mv ${IMGNAME}.tar.gz gs://${XFERBUCKET}
    # Make sure image is cleaned up if script fails
    CLEAN_IMAGES="$CLEAN_IMAGES
        ${IMGNAME}"
    echo "Importing image $IMGNAME from bucket $XFERBUCKET"
    ooe.sh gcloud compute images create \
        --source-uri gs://${XFERBUCKET}/${IMGNAME}.tar.gz \
        --family=${IMAGE_PREFIX} \
        ${IMGNAME}
    # Provide feedback of successfully imported images
    echo "${IMGNAME}" >> "$REMOTE_IMAGE_NAME_FILE"
    echo "Finished with ${IMGNAME}"
    echo ""
}

# Given an image-file name, return a GCE-friendly image name [a-z0-9-]
# N/B: Assumes image name ends in '.x86_64.raw.xz'
image_prefix(){
    req_env_var "1 $1"
    echo "$(basename $1)" | \
        tr -d '[[:space:]]' | \
        sed -r -e 's/\.x86_64\.raw\.xz//' | \
        tr '[[:upper:]]' '[[:lower:]]' | \
        tr '[[:punct:]]' '-'
}

IMAGE_PREFIX=$(image_prefix $FEDORA_IMAGE_URL)
IMAGE_URL=$FEDORA_IMAGE_URL
CSUM_URL=$FEDORA_CSUM_URL
SCRIPT="${GOSRC}/${PACKER_BASE}/fedora_base_setup.sh"
base_image_script

IMAGE_PREFIX=$(image_prefix $RHEL_IMAGE_BASENAME)
IMAGE_URL=$GOSRC/$RHEL_IMAGE_BASENAME
CSUM_URL=$GOSRC/${RHEL_IMAGE_BASENAME}.SHA256SUM
SCRIPT="${GOSRC}/${PACKER_BASE}/rhel_base_setup.sh"
base_image_script

# IMAGE_PREFIX=$(image_prefix $IMAGE_PREFIX)
# IMAGE_URL=$FAH_IMAGE_URL
# CSUM_URL=$FAH_CSUM_URL
# SCRIPT="${GOSRC}/${PACKER_BASE}/fah_base_setup.sh"
# base_image_script

rh_finalize

echo "SUCCESS!"
