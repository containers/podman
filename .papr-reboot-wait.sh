#!/usr/bin/bash
set -xeuo pipefail
host=$1

ssh_args="-o UserKnownHostsFile=/dev/null -oStrictHostKeyChecking=no -o GSSAPIAuthentication=no root@${host}"
dossh() {
    ssh ${ssh_args} "$@"
}
get_bootid() {
    dossh cat /proc/sys/kernel/random/boot_id
}

# The bootid approach is taken from rpm-ostree's libvm.sh;
# the key problem we're trying to avoid is racing back in
# via ssh to the system as it's shutting down.
bootid=$(get_bootid)

dossh systemctl reboot || true

for x in $(seq 10); do
    # Be more verbose on the last attempt
    new_bootid=$(get_bootid 2>/dev/null || true)
    if test -n "${new_bootid}" && test ${new_bootid} != ${bootid}; then
       exit 0
    fi
    sleep 3
done
echo "Failed to wait for $host; trying one more time with verbose for debugging"
ssh -vv ${ssh_args} true
exit 1
