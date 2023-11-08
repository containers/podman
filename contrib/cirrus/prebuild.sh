#!/bin/bash

set -eo pipefail

# This script attempts to confirm functional networking and
# connectivity to essential external servers.  It also verifies
# some basic environmental expectations and shell-script sanity.
# It's intended for use early on in the podman CI system, to help
# prevent wasting time on tests that can't succeed due to some
# outage, failure, or missed expectation.

set -a
source /etc/automation_environment
source $AUTOMATION_LIB_PATH/common_lib.sh
set +a

req_env_vars CI DEST_BRANCH IMAGE_SUFFIX TEST_FLAVOR TEST_ENVIRON \
             PODBIN_NAME PRIV_NAME DISTRO_NV AUTOMATION_LIB_PATH \
             SCRIPT_BASE CIRRUS_WORKING_DIR FEDORA_NAME \
             VM_IMAGE_NAME

# Defined by the CI system
# shellcheck disable=SC2154
cd $CIRRUS_WORKING_DIR

msg "Checking Cirrus YAML"
# Defined by CI config.
# shellcheck disable=SC2154
showrun $SCRIPT_BASE/cirrus_yaml_test.py

msg "Checking for leading tabs in system tests"
if grep -n ^$'\t' test/system/*; then
    die "Found leading tabs in system tests. Use spaces to indent, not tabs."
fi

# Lookup 'env' dict. string value from key specified as argument from YAML file.
get_env_key() {
    local yaml
    local script

    yaml="$CIRRUS_WORKING_DIR/.github/workflows/scan-secrets.yml"
    script="from yaml import safe_load; print(safe_load(open('$yaml'))['env']['$1'])"
    python -c "$script"
}

# Only need to check CI-stuffs on a single build-task, there's only ever
# one prior-fedora task so use that one.
# Envars all defined by CI config.
# shellcheck disable=SC2154
if [[ "${DISTRO_NV}" == "$PRIOR_FEDORA_NAME" ]]; then
    msg "Checking shell scripts"
    showrun ooe.sh dnf install -y ShellCheck  # small/quick addition
    showrun shellcheck --format=tty \
        --shell=bash --external-sources \
        --enable add-default-case,avoid-nullary-conditions,check-unassigned-uppercase \
        --exclude SC2046,SC2034,SC2090,SC2064 \
        --wiki-link-count=0 --severity=warning \
        $SCRIPT_BASE/*.sh \
        ./.github/actions/check_cirrus_cron/* \
        hack/get_ci_vm.sh

    # Tests for lib.sh
    showrun ${SCRIPT_BASE}/lib.sh.t

    # Run this during daily cron job to prevent a GraphQL API change/breakage
    # from impacting every PR.  Down-side being if it does fail, a maintainer
    # will need to do some archaeology to find it.
    # Defined by CI system
    # shellcheck disable=SC2154
    if [[ "$CIRRUS_CRON" == "main" ]]; then
      export PREBUILD=1
      showrun bash ${CIRRUS_WORKING_DIR}/.github/actions/check_cirrus_cron/test.sh
    fi

    # Note: This may detect leaks, but should not be considered authoritative
    # since any PR could modify the contents or arguments.  This check is
    # simply here to...
    msg "Checking GitLeaks functions with current CLI args, configuration, and baseline JSON"

    # TODO: Workaround for GHA Environment, duplicate here for consistency.
    # Replace with `--userns=keep-id:uid=1000,gid=1000` w/ newer podman in GHA environment.
    declare -a workaround_args
    workaround_args=(\
      --user 1000:1000
      --uidmap 0:1:1000
      --uidmap 1000:0:1
      --uidmap 1001:1001:64536
      --gidmap 0:1:1000
      --gidmap 1000:0:1
      --gidmap 1001:1001:64536
    )

    brdepth=$(get_env_key 'brdepth')
    glfqin=$(get_env_key 'glfqin')
    glargs=$(get_env_key 'glargs')
    showrun podman run --rm \
        --security-opt=label=disable \
        "${workaround_args[@]}" \
        -v $CIRRUS_WORKING_DIR:/subject:ro \
        -v $CIRRUS_WORKING_DIR:/default:ro \
        --tmpfs /report:rw,size=256k,mode=1777 \
        $glfqin \
        detect \
        --log-opts=-$brdepth \
        $glargs
fi

msg "Checking 3rd party network service connectivity"
# shellcheck disable=SC2154
cat ${CIRRUS_WORKING_DIR}/${SCRIPT_BASE}/required_host_ports.txt | \
    while read host port
    do
        if [[ "$port" -eq "443" ]]
        then
            echo "SSL/TLS to $host:$port"
            echo -n '' | \
                err_retry 9 1000 "" openssl s_client -quiet -no_ign_eof -connect $host:$port
        else
            echo "Connect to $host:$port"
            err_retry 9 1000 1 nc -zv -w 13 $host $port
        fi
    done

# Verify we can pull metadata from a few key testing images on quay.io
# in the 'libpod' namespace.  This is mostly aimed at validating the
# quay.io service is up and responsive.  Images were hand-picked with
# grep -E -ro 'quay.io/libpod/.+:latest' test | sort -u
TEST_IMGS=(\
    alpine:latest
    busybox:latest
    alpine_labels:latest
    alpine_nginx:latest
    alpine_healthcheck:latest
    badhealthcheck:latest
    cirros:latest
)

msg "Checking quay.io test image accessibility"
for testimg in "${TEST_IMGS[@]}"; do
    fqin="quay.io/libpod/$testimg"
    echo "    $fqin"
    # Belt-and-suspenders: Catch skopeo (somehow) returning False or null
    # in addition to "bad" (invalid) JSON.
    skopeo inspect --retry-times 5 "docker://$fqin" | jq -e . > /dev/null
done
