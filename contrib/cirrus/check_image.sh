#!/usr/bin/env bash

set -eo pipefail

source $(dirname $0)/lib.sh

EVIL_UNITS="$($CIRRUS_WORKING_DIR/$PACKER_BASE/systemd_banish.sh --list)"

req_env_var PACKER_BUILDER_NAME RCLI EVIL_UNITS OS_RELEASE_ID CG_FS_TYPE

NFAILS=0
echo "Validating VM image"

MIN_SLASH_GIGS=30
read SLASH_DEVICE SLASH_FSTYPE SLASH_SIZE JUNK <<<$(findmnt --df --first-only --noheadings / | cut -d '.' -f 1)
SLASH_SIZE_GIGS=$(echo "$SLASH_SIZE" | sed -r -e 's/G|g//')
item_test "Minimum available disk space" $SLASH_SIZE_GIGS -gt $MIN_SLASH_GIGS || let "NFAILS+=1"

MIN_MEM_MB=2000
read JUNK TOTAL USED MEM_FREE JUNK <<<$(free -tm | tail -1)
item_test 'Minimum available memory' $MEM_FREE -ge $MIN_MEM_MB || let "NFAILS+=1"

# We're testing a custom-built podman; make sure there isn't a distro-provided
# binary anywhere; that could potentially taint our results.
remove_packaged_podman_files
item_test "remove_packaged_podman_files() does it's job" -z "$(type -P podman)" || let "NFAILS+=1"

MIN_ZIP_VER='3.0'
VER_RE='.+([[:digit:]]+\.[[:digit:]]+).+'
ACTUAL_VER=$(zip --version 2>&1 | egrep -m 1 "Zip$VER_RE" | sed -r -e "s/$VER_RE/\\1/")
item_test "minimum zip version" "$MIN_ZIP_VER" = $(echo -e "$MIN_ZIP_VER\n$ACTUAL_VER" | sort -V | head -1) || let "NFAILS+=1"

for REQ_UNIT in google-accounts-daemon.service \
                google-clock-skew-daemon.service \
                google-instance-setup.service \
                google-network-daemon.service \
                google-shutdown-scripts.service \
                google-startup-scripts.service
do
    # enabled/disabled appears at the end of the line, on some Ubuntu's it appears twice
    service_status=$(systemctl list-unit-files --no-legend $REQ_UNIT | tac -s ' ' | head -1)
    item_test "required $REQ_UNIT status is enabled" \
        "$service_status" = "enabled" || let "NFAILS+=1"
done

for evil_unit in $EVIL_UNITS
do
    # Exits zero if any unit matching pattern is running
    unit_status=$(systemctl is-active $evil_unit &> /dev/null; echo $?)
    item_test "No $evil_unit unit is present or active:" "$unit_status" -ne "0" || let "NFAILS+=1"
done

echo "Checking items specific to ${PACKER_BUILDER_NAME}${BUILT_IMAGE_SUFFIX}"
case "$PACKER_BUILDER_NAME" in
    ubuntu*)
        item_test "On ubuntu, no periodic apt crap is enabled" -z "$(egrep $PERIODIC_APT_RE /etc/apt/apt.conf.d/*)"
        ;;
    fedora*)
        # Only runc -OR- crun should be installed, never both
        case "$CG_FS_TYPE" in
            tmpfs)
                HAS=runc
                HAS_NOT=crun
                ;;
            cgroup2fs)
                HAS=crun
                HAS_NOT=runc
                ;;
        esac
        HAS_RC=$(rpm -qV $HAS &> /dev/null; echo $?)
        HAS_NOT_RC=$(rpm -qV $HAS_NOT &> /dev/null; echo $?)
        item_test "With a cgroups-fs type $CG_FS_TYPE, the $HAS package is installed" $HAS_RC -eq 0
        item_test "With a cgroups-fs type $CG_FS_TYPE, the $HAS_NOT package is not installed" $HAS_NOT_RC -ne 0
        ;;
    xfedora*)
        echo "Kernel Command-line: $(cat /proc/cmdline)"
        item_test \
            "On ${PACKER_BUILDER_NAME} images, the /sys/fs/cgroup/unified directory does NOT exist" \
            "!" "-d" "/sys/fs/cgroup/unified" || let "NFAILS+=1"
        ;;
    *) echo "No vm-image specific items to check"
esac

echo "Total failed tests: $NFAILS"
exit $NFAILS
