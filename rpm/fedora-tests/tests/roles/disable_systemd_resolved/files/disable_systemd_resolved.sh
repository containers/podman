#!/bin/bash
#
# Excerpted from https://github.com/containers/automation_images/blob/main/systemd_banish.sh
#
# Early 2023: https://github.com/containers/podman/issues/16973
#
# We see countless instances of "lookup cdn03.quay.io" flakes.
# Disabling the systemd resolver has completely resolved those,
# from multiple flakes per day to zero in a month.
#
# Opinions differ on the merits of systemd-resolve, but the fact is
# it breaks our CI testing. Kill it.
nsswitch=/etc/authselect/nsswitch.conf
if [[ -e $nsswitch ]]; then
    if grep -q -E 'hosts:.*resolve' $nsswitch; then
        echo "Disabling systemd-resolved"
        sed -i -e 's/^\(hosts: *\).*/\1files dns myhostname/' $nsswitch
        systemctl disable --now systemd-resolved
        rm -f /etc/resolv.conf

        # NetworkManager may already be running, or it may not....
        systemctl start NetworkManager
        sleep 1
        systemctl restart NetworkManager

        # ...and it may create resolv.conf upon start/restart, or it
        # may not. Keep restarting until it does. (Yes, I realize
        # this is cargocult thinking. Don't care. Not worth the effort
        # to diagnose and solve properly.)
        retries=10
        while ! test -e /etc/resolv.conf;do
              retries=$((retries - 1))
              if [[ $retries -eq 0 ]]; then
                  echo "Timed out waiting for resolv.conf" >&2
                  echo "...gonna try continuing. Expect failures." >&2
              fi
              systemctl restart NetworkManager
              sleep 5
        done
    fi
fi
