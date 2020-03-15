#!/bin/bash

set +e  # Not all of these exist on every platform

# This is intended to be executed on VMs as a startup script on initial-boot.
# Alternatively, it may be executed with the '--list' option to return the list
# of systemd units defined for disablement (useful for testing).

EVIL_UNITS="cron crond atd apt-daily-upgrade apt-daily fstrim motd-news systemd-tmpfiles-clean"

if [[ "$1" == "--list" ]]
then
    echo "$EVIL_UNITS"
    exit 0
fi

echo "Disabling periodic services that could destabilize testing:"
for unit in $EVIL_UNITS
do
    echo "Banishing $unit (ignoring errors)"
    (
        sudo systemctl stop $unit
        sudo systemctl disable $unit
        sudo systemctl disable $unit.timer
        sudo systemctl mask $unit
        sudo systemctl mask $unit.timer
    ) &> /dev/null
done
