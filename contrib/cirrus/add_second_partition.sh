#!/usr/bin/env bash

# N/B: This script could mega f*!@up your disks if run by mistake.
#      it is left without the execute-bit on purpose!

set -eo pipefail

# shellcheck source=./lib.sh
source $(dirname $0)/lib.sh

# $SLASH_DEVICE is the disk device to be f*xtuP
SLASH_DEVICE="/dev/sda"  # Always the case on GCP

# The unallocated space results from the difference in disk-size between VM Image
# and runtime request.
NEW_PART_START="50%"
NEW_PART_END="100%"


if [[ ! -r "/root" ]] || [[ -r "/root/second_partition_ready" ]]
then
    warn "Ignoring attempted execution of $(basename $0)"
    exit 0
fi

[[ -x "$(type -P parted)" ]] || \
    die "The parted command is required."

[[ ! -b ${SLASH_DEVICE}2 ]] || \
    die "Found unexpected block device ${SLASH_DEVICE}2"

PPRINTCMD="parted --script ${SLASH_DEVICE} print"
FINDMNTCMD="findmnt --source=${SLASH_DEVICE}1 --mountpoint=/ --canonicalize --evaluate --first-only --noheadings"
TMPF=$(mktemp -p '' $(basename $0)_XXXX)
trap "rm -f $TMPF" EXIT

if $FINDMNTCMD | tee $TMPF | egrep -q "^/\s+${SLASH_DEVICE}1"
then
    msg "Repartitioning original partition table:"
    $PPRINTCMD
else
    die "Unexpected output from '$FINDMNTCMD': $(<$TMPF)"
fi

echo "Adding partition offset within unpartitioned space."
parted --script --align optimal /dev/sda unit % mkpart primary "" "" "$NEW_PART_START" "$NEW_PART_END"

msg "New partition table:"
$PPRINTCMD

msg "Growing ${SLASH_DEVICE}1 meet start of ${SLASH_DEVICE}2"
growpart ${SLASH_DEVICE} 1

FSTYPE=$(findmnt --first-only --noheadings --output FSTYPE ${SLASH_DEVICE}1)
echo "Expanding $FSTYPE filesystem on ${SLASH_DEVICE}1"
case $FSTYPE in
    ext*) resize2fs ${SLASH_DEVICE}1 ;;
    *) die "Script $(basename $0) doesn't know how to resize a $FSTYPE filesystem." ;;
esac

# Must happen last - signals completion to other tooling
msg "Recording newly available disk partition device into /root/second_partition_ready"
echo "${SLASH_DEVICE}2" > /root/second_partition_ready
