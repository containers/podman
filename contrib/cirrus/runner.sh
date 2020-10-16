#!/bin/bash

set -eo pipefail

# This script runs in the Cirrus CI environment, invoked from .cirrus.yml .
# It can also be invoked manually in a `hack/get_ci_cm.sh` environment,
# documentation of said usage is TBI.
#
# The principal deciding factor is the $TEST_FLAVOR envariable: for any
# given value 'xyz' there must be a function '_run_xyz' to handle that
# test. Several other envariables are used to differentiate further,
# most notably:
#
#    PODBIN_NAME  : "podman" (i.e. local) or "remote"
#    TEST_ENVIRON : 'host' or 'container'; desired environment in which to run
#    CONTAINER    : 1 if *currently* running inside a container, 0 if host
#

# shellcheck source=contrib/cirrus/lib.sh
source $(dirname $0)/lib.sh

function _run_ext_svc() {
    $SCRIPT_BASE/ext_svc_check.sh
}

function _run_smoke() {
    make gofmt

    # There is little value to validating commits after tag-push
    # and it's very difficult to automatically determine a starting commit.
    # $CIRRUS_TAG is only non-empty when executing due to a tag-push
    # shellcheck disable=SC2154
    if [[ -z "$CIRRUS_TAG" ]]; then
        make .gitvalidation
    fi
}

function _run_automation() {
    $SCRIPT_BASE/cirrus_yaml_test.py

    req_env_vars CI DEST_BRANCH IMAGE_SUFFIX TEST_FLAVOR TEST_ENVIRON \
                 PODBIN_NAME PRIV_NAME DISTRO_NV CONTAINER USER HOME \
                 UID AUTOMATION_LIB_PATH SCRIPT_BASE OS_RELEASE_ID \
                 OS_RELEASE_VER CG_FS_TYPE
    bigto ooe.sh dnf install -y ShellCheck  # small/quick addition
    $SCRIPT_BASE/shellcheck.sh
}

function _run_validate() {
    # Confirm compile via prior task + cache
    bin/podman --version
    bin/podman-remote --version
    make validate  # Some items require a build
}

function _run_unit() {
    # shellcheck disable=SC2154
    if [[ "$PODBIN_NAME" != "podman" ]]; then
        # shellcheck disable=SC2154
        die "$TEST_FLAVOR: Unsupported PODBIN_NAME='$PODBIN_NAME'"
    fi
    make localunit
}

function _run_apiv2() {
    make localapiv2 |& logformatter
}

function _run_int() {
    dotest integration
}

function _run_sys() {
    dotest system
}

function _run_bindings() {
    # shellcheck disable=SC2155
    export PATH=$PATH:$GOSRC/hack

    # Subshell needed so logformatter will write output in cwd; if it runs in
    # the subdir, .cirrus.yml will not find the html'ized log
    (cd pkg/bindings/test && ginkgo -trace -noColor -debug  -r) |& logformatter
}

function _run_docker-py() {
    msg "This is docker-py stub, it is only a stub"
}

function _run_endpoint() {
    make test-binaries
    make endpoint
}

exec_container() {
    local var_val
    local cmd
    # Required to be defined by caller
    # shellcheck disable=SC2154
    msg "Re-executing runner inside container: $CTR_FQIN"
    msg "************************************************************"

    req_env_vars CTR_FQIN TEST_ENVIRON CONTAINER SECRET_ENV_RE

    # Line-separated arguments which include shell-escaped special characters
    declare -a envargs
    while read -r var_val; do
        envargs+=("-e $var_val")
    done <<<"$(passthrough_envars)"

    # VM Images and Container images are built using (nearly) identical operations.
    set -x
    # shellcheck disable=SC2154
    exec podman run --rm --privileged --net=host --cgroupns=host \
        -v /dev/fuse:/dev/fuse \
        -v "$GOPATH:$GOPATH:Z" \
        --workdir "$GOSRC" \
        -e "CONTAINER=1" \
        "${envargs[@]}" \
        $CTR_FQIN bash -c "$SCRIPT_BASE/setup_environment.sh && $SCRIPT_BASE/runner.sh"
}

function _run_swagger() {
    local download_url
    # Building this is a PITA, just grab binary for use in automation
    # Ref: https://goswagger.io/install.html#static-binary
    download_url=$(\
        curl -s https://api.github.com/repos/go-swagger/go-swagger/releases/latest | \
        jq -r '.assets[] | select(.name | contains("linux_amd64")) | .browser_download_url')
    curl -o /usr/local/bin/swagger -L'#' "$download_url"
    chmod +x /usr/local/bin/swagger

    cd $GOSRC
    make swagger

    # Cirrus-CI Artifact instruction expects file here
    cp -v $GOSRC/pkg/api/swagger.yaml $GOSRC/
}

function _run_vendor() {
    make vendor
    ./hack/tree_status.sh
}

function _run_build() {
    # Ensure always start from clean-slate with all vendor modules downloaded
    make clean
    make vendor
    make podman-release
    make podman-remote-linux-release
}

function _run_altbuild() {
    req_env_vars ALT_NAME
    # Defined in .cirrus.yml
    # shellcheck disable=SC2154
    msg "Performing alternate build: $ALT_NAME"
    msg "************************************************************"
    cd $GOSRC
    case "$ALT_NAME" in
        *Each*)
            git fetch origin
            make build-all-new-commits GIT_BASE_BRANCH=origin/$DEST_BRANCH
            ;;
        *Windows*)
            make podman-remote-windows-release
            make podman.msi
            ;;
        *Without*)
            make build-no-cgo
            ;;
        *varlink-API)
            export SUGGESTION='remove API.md, then "make varlink_api_generate" and commit changes.'
            make varlink_api_generate BUILDTAGS="varlink"
            ./hack/tree_status.sh
            ;;
        *varlink-binaries)
            make clean BUILDTAGS="varlink" binaries
            ;;
        *RPM*)
            make -f ./.copr/Makefile
            rpmbuild --rebuild ./podman-*.src.rpm
            ;;
        *Static*)
            req_env_vars CTR_FQIN
            [[ "$UID" -eq 0 ]] || \
                die "Static build must execute nixos container as root on host"
            mkdir -p /var/cache/nix
            podman run -i --rm -v /var/cache/nix:/mnt/nix:Z \
                $CTR_FQIN cp -rfT /nix /mnt/nix
            podman run -i --rm -v /var/cache/nix:/nix:Z \
                -v $PWD:$PWD:Z -w $PWD $CTR_FQIN \
                nix --print-build-logs --option cores 4 --option max-jobs 4 \
                    build --file ./nix/
            # result symlink is absolute from container perspective :(
            cp /var/cache/$(readlink result)/bin/podman ./  # for cirrus-ci artifact
            rm result  # makes cirrus puke
            ;;
        *)
            die "Unknown/Unsupported \$$ALT_NAME '$ALT_NAME'"
    esac
}

function _run_release() {
    if bin/podman info |& grep -Eq -- '-dev'; then
        die "Releases must never contain '-dev' in output of 'podman info'"
    fi
}

logformatter() {
    # Use similar format as human-friendly task name from .cirrus.yml
    # shellcheck disable=SC2154
    output_name="$TEST_FLAVOR-$PODBIN_NAME-$DISTRO_NV-$PRIV_NAME-$TEST_ENVIRON"
    # Requires stdin and stderr combined!
    cat - \
        |& awk --file "${CIRRUS_WORKING_DIR}/${SCRIPT_BASE}/timestamp.awk" \
        |& "${CIRRUS_WORKING_DIR}/${SCRIPT_BASE}/logformatter" "$output_name"
}

# Handle local|remote integration|system testing in a uniform way
dotest() {
    local testsuite="$1"
    req_env_vars testsuite CONTAINER TEST_ENVIRON PRIV_NAME

    # shellcheck disable=SC2154
    if ((CONTAINER==0)) && [[ "$TEST_ENVIRON" == "container" ]]; then
        exec_container  # does not return
    fi;

    # shellcheck disable=SC2154
    if [[ "$PRIV_NAME" == "rootless" ]] && [[ "$UID" -eq 0 ]]; then
        req_env_vars ROOTLESS_USER
        msg "Re-executing runner through ssh as user '$ROOTLESS_USER'"
        msg "************************************************************"
        set -x
        exec ssh $ROOTLESS_USER@localhost \
                -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no \
                -o CheckHostIP=no $GOSRC/$SCRIPT_BASE/runner.sh
        # does not return
    fi

    # containers/automation sets this to 0 for its dbg() function
    # but the e2e integration tests are also sensitive to it.
    unset DEBUG

    # shellcheck disable=SC2154
    local localremote="$PODBIN_NAME"
    case "$PODBIN_NAME" in
        podman)  localremote="local" ;;
    esac

    make ${localremote}${testsuite} PODMAN_SERVER_LOG=$PODMAN_SERVER_LOG \
        |& logformatter
}

msg "************************************************************"
# Required to be defined by caller
# shellcheck disable=SC2154
msg "Runner executing $TEST_FLAVOR $PODBIN_NAME-tests as $PRIV_NAME on $DISTRO_NV($OS_REL_VER)"
if ((CONTAINER)); then
    # shellcheck disable=SC2154
    msg "Current environment container image: $CTR_FQIN"
else
    # shellcheck disable=SC2154
    msg "Current environment VM image: $VM_IMAGE_NAME"
fi
msg "************************************************************"

((${SETUP_ENVIRONMENT:-0})) || \
    die "Expecting setup_environment.sh to have completed successfully"

cd "${GOSRC}/"

handler="_run_${TEST_FLAVOR}"

if [ "$(type -t $handler)" != "function" ]; then
    die "Unknown/Unsupported \$TEST_FLAVOR=$TEST_FLAVOR"
fi

$handler
