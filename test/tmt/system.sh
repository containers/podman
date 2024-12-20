#!/usr/bin/env bash

set -exo pipefail

. setup.sh

export test_cmd="whoami && cd /usr/share/podman/test/system && bats ."

if [[ -z $1 ]]; then
    eval $test_cmd
elif [[ $1 == "rootless" ]]; then
    su --whitelist-environment=$(cat ./tmt-envvars | tr '\n' ',') - "$ROOTLESS_USER" -c "eval $test_cmd"
fi
exit 0
