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
#    TEST_ENVIRON : 'host', 'host-netavark', or 'container'; desired environment in which to run
#    CONTAINER    : 1 if *currently* running inside a container, 0 if host
#

# shellcheck source=contrib/cirrus/lib.sh
source $(dirname $0)/lib.sh

function _run_ext_svc() {
    $SCRIPT_BASE/ext_svc_check.sh
}

function _run_automation() {
    $SCRIPT_BASE/cirrus_yaml_test.py

    req_env_vars CI DEST_BRANCH IMAGE_SUFFIX TEST_FLAVOR TEST_ENVIRON \
                 PODBIN_NAME PRIV_NAME DISTRO_NV CONTAINER USER HOME \
                 UID AUTOMATION_LIB_PATH SCRIPT_BASE OS_RELEASE_ID \
                 CG_FS_TYPE
    bigto ooe.sh dnf install -y ShellCheck  # small/quick addition
    $SCRIPT_BASE/shellcheck.sh
}

function _run_validate() {
    # git-validation tool fails if $EPOCH_TEST_COMMIT is empty
    # shellcheck disable=SC2154
    if [[ -n "$EPOCH_TEST_COMMIT" ]]; then
        make validate
    else
        warn "Skipping git-validation since \$EPOCH_TEST_COMMIT is empty"
    fi

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

function _run_compose() {
    ./test/compose/test-compose |& logformatter
}

function _run_int() {
    dotest integration
}

function _run_sys() {
    dotest system
}

function _run_upgrade_test() {
    bats test/upgrade |& logformatter
}

function _run_bud() {
    ./test/buildah-bud/run-buildah-bud-tests |& logformatter
}

function _run_bindings() {
    # shellcheck disable=SC2155
    export PATH=$PATH:$GOSRC/hack

    # if logformatter sees this, it can link directly to failing source lines
    local gitcommit_magic=
    if [[ -n "$GIT_COMMIT" ]]; then
        gitcommit_magic="/define.gitCommit=${GIT_COMMIT}"
    fi

    # Subshell needed so logformatter will write output in cwd; if it runs in
    # the subdir, .cirrus.yml will not find the html'ized log
    (cd pkg/bindings/test && \
         echo "$gitcommit_magic" && \
         ginkgo -progress -trace -noColor -debug -timeout 30m -r -v) |& logformatter
}

function _run_docker-py() {
    source venv/bin/activate
    make run-docker-py-tests
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
        -v `mktemp -d -p /var/tmp`:/tmp:Z \
        -v /dev/fuse:/dev/fuse \
        -v "$GOPATH:$GOPATH:Z" \
        --workdir "$GOSRC" \
        -e "CONTAINER=1" \
        "${envargs[@]}" \
        $CTR_FQIN bash -c "$SCRIPT_BASE/setup_environment.sh && $SCRIPT_BASE/runner.sh"
}

function _run_swagger() {
    local upload_filename
    local upload_bucket
    local download_url
    local envvarsfile
    req_env_vars GCPJSON GCPNAME GCPPROJECT CTR_FQIN

    [[ -x /usr/local/bin/swagger ]] || \
        die "Expecting swagger binary to be present and executable."

    # The filename and bucket depend on the automation context
    #shellcheck disable=SC2154,SC2153
    if [[ -n "$CIRRUS_PR" ]]; then
        upload_bucket="libpod-pr-releases"
        upload_filename="swagger-pr$CIRRUS_PR.yaml"
    elif [[ -n "$CIRRUS_TAG" ]]; then
        upload_bucket="libpod-master-releases"
        upload_filename="swagger-$CIRRUS_TAG.yaml"
    elif [[ "$CIRRUS_BRANCH" == "main" ]]; then
        upload_bucket="libpod-master-releases"
        # readthedocs versioning uses "latest" for "main" (default) branch
        upload_filename="swagger-latest.yaml"
    elif [[ -n "$CIRRUS_BRANCH" ]]; then
        upload_bucket="libpod-master-releases"
        upload_filename="swagger-$CIRRUS_BRANCH.yaml"
    else
        die "Unknown execution context, expected a non-empty value for \$CIRRUS_TAG, \$CIRRUS_BRANCH, or \$CIRRUS_PR"
    fi

    # Swagger validation takes a significant amount of time
    msg "Pulling \$CTR_FQIN '$CTR_FQIN' (background process)"
    podman pull --quiet $CTR_FQIN &

    cd $GOSRC
    make swagger

    # Cirrus-CI Artifact instruction expects file here
    cp -v $GOSRC/pkg/api/swagger.yaml ./

    envvarsfile=$(mktemp -p '' .tmp_$(basename $0)_XXXXXXXX)
    trap "rm -f $envvarsfile" EXIT  # contains secrets
    # Warning: These values must _not_ be quoted, podman will not remove them.
    #shellcheck disable=SC2154
    cat <<eof >>$envvarsfile
GCPJSON=$GCPJSON
GCPNAME=$GCPNAME
GCPPROJECT=$GCPPROJECT
FROM_FILEPATH=$GOSRC/swagger.yaml
TO_GCSURI=gs://$upload_bucket/$upload_filename
eof

    msg "Waiting for backgrounded podman pull to complete..."
    wait %%
    podman run -it --rm --security-opt label=disable \
        --env-file=$envvarsfile \
        -v $GOSRC:$GOSRC:ro \
        --workdir $GOSRC \
        $CTR_FQIN
    rm -f $envvarsfile
}

function _run_consistency() {
    make vendor
    SUGGESTION="run 'make vendor' and commit all changes" ./hack/tree_status.sh
    make generate-bindings
    SUGGESTION="run 'make generate-bindings' and commit all changes" ./hack/tree_status.sh
    make completions
    SUGGESTION="run 'make completions' and commit all changes" ./hack/tree_status.sh
}

function _run_build() {
    # Ensure always start from clean-slate with all vendor modules downloaded
    make clean
    make vendor
    make podman-release  # includes podman, podman-remote, and docs
}

function _run_altbuild() {
    local -a arches
    local arch
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
            make podman-remote-release-windows_amd64.zip
            make podman.msi
            ;;
        *Without*)
            make build-no-cgo
            ;;
        *RPM*)
            make package
            ;;
        Alt*Cross)
            arches=(\
                amd64
                ppc64le
                arm
                arm64
                386
                s390x
                mips
                mipsle
                mips64
                mips64le)
            for arch in "${arches[@]}"; do
                msg "Building release archive for $arch"
                make podman-release-${arch}.tar.gz GOARCH=$arch
            done
            ;;
        *)
            die "Unknown/Unsupported \$$ALT_NAME '$ALT_NAME'"
    esac
}

function _run_release() {
    # TODO: These tests should come from code external to the podman repo.
    # to allow test-changes (and re-runs) in the case of a correctable test
    # flaw or flake at release tag-push time.  For now, the test is here
    # given its simplicity.
    msg "podman info:"
    bin/podman info

    msg "Checking podman release (or potential release) criteria."
    # We're running under 'set -eo pipefail'; make sure this statement passes
    dev=$(bin/podman info |& grep -- -dev || echo -n '')
    if [[ -n "$dev" ]]; then
        die "Releases must never contain '-dev' in output of 'podman info' ($dev)"
    fi
    msg "All OK"
}


function _run_gitlab() {
    rootless_uid=$(id -u)
    systemctl enable --now --user podman.socket
    export DOCKER_HOST=unix:///run/user/${rootless_uid}/podman/podman.sock
    export CONTAINER_HOST=$DOCKER_HOST
    cd $GOPATH/src/gitlab.com/gitlab-org/gitlab-runner
    set +e
    go test -v ./executors/docker |& tee $GOSRC/gitlab-runner-podman.log
    ret=$?
    set -e
    # This file is collected and parsed by Cirrus-CI so must be in $GOSRC
    cat $GOSRC/gitlab-runner-podman.log | \
        go-junit-report > $GOSRC/gitlab-runner-podman.xml
    return $ret
}

logformatter() {
    if [[ "$CI" == "true" ]]; then
        # Use similar format as human-friendly task name from .cirrus.yml
        # shellcheck disable=SC2154
        output_name="$TEST_FLAVOR-$PODBIN_NAME-$DISTRO_NV-$PRIV_NAME-$TEST_ENVIRON"
        # Requires stdin and stderr combined!
        cat - \
            |& awk --file "${CIRRUS_WORKING_DIR}/${SCRIPT_BASE}/timestamp.awk" \
            |& "${CIRRUS_WORKING_DIR}/${SCRIPT_BASE}/logformatter" "$output_name"
    else
        # Assume script is run by a human, they want output immediately
        cat -
    fi
}

# Handle local|remote integration|system testing in a uniform way
dotest() {
    local testsuite="$1"
    req_env_vars testsuite CONTAINER TEST_ENVIRON PRIV_NAME

    # shellcheck disable=SC2154
    if ((CONTAINER==0)) && [[ "$TEST_ENVIRON" == "container" ]]; then
        exec_container  # does not return
    fi;

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

# Nearly every task in .cirrus.yml makes use of this shell script
# wrapped by /usr/bin/time to collect runtime statistics.  Because the
# --output option is used to log stats to a file, every child-process
# inherits an open FD3 pointing at the log.  However, some testing
# operations depend on making use of FD3, and so it must be explicitly
# closed here (and for all further child-processes).
# STATS_LOGFILE assumed empty/undefined outside of Cirrus-CI (.cirrus.yml)
# shellcheck disable=SC2154
exec 3<&-

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

# shellcheck disable=SC2154
if [[ "$PRIV_NAME" == "rootless" ]] && [[ "$UID" -eq 0 ]]; then
    # Remove /var/lib/cni, it is not required for rootless cni.
    # We have to test that it works without this directory.
    # https://github.com/containers/podman/issues/10857
    rm -rf /var/lib/cni

    req_env_vars ROOTLESS_USER
    msg "Re-executing runner through ssh as user '$ROOTLESS_USER'"
    msg "************************************************************"
    set -x
    exec ssh $ROOTLESS_USER@localhost \
            -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no \
            -o CheckHostIP=no $GOSRC/$SCRIPT_BASE/runner.sh
    # Does not return!
fi
# else: not running rootless, do nothing special

cd "${GOSRC}/"

handler="_run_${TEST_FLAVOR}"

if [ "$(type -t $handler)" != "function" ]; then
    die "Unknown/Unsupported \$TEST_FLAVOR=$TEST_FLAVOR"
fi

$handler
