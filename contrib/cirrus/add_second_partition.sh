#!/bin/bash

# N/B: This script could mega f*!@up your disks if run by mistake.
#      it is left without the execute-bit on purpose!

# $SLASH_DEVICE is the disk device to be f*xtuP
SLASH_DEVICE="/dev/sda"  # Always the case on GCP

# The unallocated space results from the difference in disk-size between VM Image
# and runtime request.  The check_image.sh test includes a minimum-space check,
# with the Image size set initially lower by contrib/cirrus/packer/libpod_images.yml
NEW_PART_START="50%"
NEW_PART_END="100%"

set -eo pipefail

source $(dirname $0)/lib.sh

if [[ ! -r "/root" ]] || [[ -r "/root/second_partition_ready" ]]
then
    echo "Warning: Ignoring attempted execution of $(basename $0)"
    exit 0
fi

[[ -n "type -P parted" ]] || \
    die 2 "The parted command is required."

[[ ! -b ${SLASH_DEVICE}2 ]] || \
    die 5 "Found unexpected block device ${SLASH_DEVICE}2"

PPRINTCMD="parted --script ${SLASH_DEVICE} print"
FINDMNTCMD="findmnt --source=${SLASH_DEVICE}1 --mountpoint=/ --canonicalize --evaluate --first-only --noheadings"
TMPF=$(mktemp -p '' $(basename $0)_XXXX)
trap "rm -f $TMPF" EXIT

if $FINDMNTCMD | tee $TMPF | egrep -q "^/\s+${SLASH_DEVICE}1"
then
    echo "Repartitioning original partition table:"
    $PPRINTCMD
else
    die 6 "Unexpected output from '$FINDMNTCMD': $(<$TMPF)"
fi

echo "Adding partition offset within unpartitioned space."
parted --script --align optimal /dev/sda unit % mkpart primary "" "" "$NEW_PART_START" "$NEW_PART_END"

echo "New partition table:"
$PPRINTCMD

echo "Growing ${SLASH_DEVICE}1 meet start of ${SLASH_DEVICE}2"
growpart ${SLASH_DEVICE} 1

FSTYPE=$(findmnt --first-only --noheadings --output FSTYPE ${SLASH_DEVICE}1)
echo "Expanding $FSTYPE filesystem on ${SLASH_DEVICE}1"
case $FSTYPE in
    ext*) resize2fs ${SLASH_DEVICE}1 ;;
    *) die 11 "Script $(basename $0) doesn't know how to resize a $FSTYPE filesystem." ;;
esac

# Must happen last - signals completion to other tooling
echo "Recording newly available disk partition device into /root/second_partition_ready"
echo "${SLASH_DEVICE}2" > /root/second_partition_ready
