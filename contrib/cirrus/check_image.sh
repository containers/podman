#!/bin/bash

set -eo pipefail

source $(dirname $0)/lib.sh

RET=0
echo "Validating VM image"

MIN_SLASH_GIGS=50
read SLASH_DEVICE SLASH_FSTYPE SLASH_SIZE JUNK <<<$(findmnt --df --first-only --noheadings / | cut -d '.' -f 1)
SLASH_SIZE_GIGS=$(echo "$SLASH_SIZE" | sed -r -e 's/G|g//')
item_test "Minimum available disk space" $SLASH_SIZE_GIGS -gt $MIN_SLASH_GIGS || let "RET+=1"

MIN_MEM_MB=2000
read JUNK TOTAL USED MEM_FREE JUNK <<<$(free -tm | tail -1)
item_test 'Minimum available memory' $MEM_FREE -ge $MIN_MEM_MB || let "RET+=1"

# We're testing a custom-built podman; make sure there isn't a distro-provided
# binary anywhere; that could potentially taint our results.
item_test "remove_packaged_podman_files() did it's job" -z "$(type -P podman)" || let "RET+=1"

MIN_ZIP_VER='3.0'
VER_RE='.+([[:digit:]]+\.[[:digit:]]+).+'
ACTUAL_VER=$(zip --version 2>&1 | egrep -m 1 "Zip$VER_RE" | sed -r -e "s/$VER_RE/\\1/")
item_test "minimum zip version" "$MIN_ZIP_VER" = $(echo -e "$MIN_ZIP_VER\n$ACTUAL_VER" | sort -V | head -1) || let "RET+=1"

for REQ_UNIT in google-accounts-daemon.service \
                google-clock-skew-daemon.service \
                google-instance-setup.service \
                google-network-daemon.service \
                google-shutdown-scripts.service \
                google-startup-scripts.service
do
    item_test "required $REQ_UNIT enabled" \
        "$(systemctl list-unit-files --no-legend $REQ_UNIT)" = "$REQ_UNIT enabled" || let "RET+=1"
done

exit $RET
