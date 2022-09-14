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
#    TEST_ENVIRON : 'host', or 'container'; desired environment in which to run
#    CONTAINER    : 1 if *currently* running inside a container, 0 if host
#

# shellcheck source=contrib/cirrus/lib.sh
source $(dirname $0)/lib.sh

function _run_validate() {
    # TODO: aarch64 images need python3-devel installed
    # https://github.com/containers/automation_images/issues/159
    bigto ooe.sh dnf install -y python3-devel

    # git-validation tool fails if $EPOCH_TEST_COMMIT is empty
    # shellcheck disable=SC2154
    if [[ -n "$EPOCH_TEST_COMMIT" ]]; then
        make validate
    else
        warn "Skipping git-validation since \$EPOCH_TEST_COMMIT is empty"
    fi

}

function _run_unit() {
    _bail_if_test_can_be_skipped test/goecho test/version

    # shellcheck disable=SC2154
    if [[ "$PODBIN_NAME" != "podman" ]]; then
        # shellcheck disable=SC2154
        die "$TEST_FLAVOR: Unsupported PODBIN_NAME='$PODBIN_NAME'"
    fi
    make localunit
}

function _run_apiv2() {
    _bail_if_test_can_be_skipped test/apiv2

    (
        make localapiv2-bash
        source .venv/requests/bin/activate
        make localapiv2-python
    ) |& logformatter
}

function _run_compose() {
    _bail_if_test_can_be_skipped test/compose

    ./test/compose/test-compose |& logformatter
}

function _run_compose_v2() {
    _bail_if_test_can_be_skipped test/compose

    ./test/compose/test-compose |& logformatter
}

function _run_int() {
    _bail_if_test_can_be_skipped test/e2e

    dotest integration
}

function _run_sys() {
    _bail_if_test_can_be_skipped test/system

    dotest system
}

function _run_upgrade_test() {
    _bail_if_test_can_be_skipped test/upgrade

    bats test/upgrade |& logformatter
}

function _run_bud() {
    _bail_if_test_can_be_skipped test/buildah-bud

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
    source .venv/docker-py/bin/activate
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
        # Pass "-e VAR" on the command line, not "-e VAR=value". Podman can
        # do a much better job of transmitting the value than we can,
        # especially when value includes spaces.
        envargs+=("-e" "$(awk -F= '{print $1}' <<<$var_val)")
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

function _run_build() {
    # Ensure always start from clean-slate with all vendor modules downloaded
    make clean
    make vendor
    make podman-release  # includes podman, podman-remote, and docs

    # Last-minute confirmation that we're testing the desired runtime.
    # This Can't Possibly Fail™ in regular CI; only when updating VMs.
    # $CI_DESIRED_RUNTIME must be defined in .cirrus.yml.
    req_env_vars CI_DESIRED_RUNTIME
    runtime=$(bin/podman info --format '{{.Host.OCIRuntime.Name}}')
    # shellcheck disable=SC2154
    if [[ "$runtime" != "$CI_DESIRED_RUNTIME" ]]; then
        die "Built podman is using '$runtime'; this CI environment requires $CI_DESIRED_RUNTIME"
    fi
    msg "Built podman is using expected runtime='$runtime'"
}

function _run_altbuild() {
    # We can skip all these steps for test-only PRs, but not doc-only ones
    _bail_if_test_can_be_skipped docs

    local -a arches
    local arch
    req_env_vars ALT_NAME
    # Defined in .cirrus.yml
    # shellcheck disable=SC2154
    msg "Performing alternate build: $ALT_NAME"
    msg "************************************************************"
    set -x
    cd $GOSRC
    case "$ALT_NAME" in
        *Each*)
            git fetch origin
            # The make-and-check-size script, introduced 2022-03-22 in #13518,
            # runs 'make' (the original purpose of this check) against
            # each commit, then checks image sizes to make sure that
            # none have grown beyond a given limit. That of course
            # requires a baseline, so our first step is to build the
            # branch point of the PR.
            local context_dir savedhead pr_base
            context_dir=$(mktemp -d --tmpdir make-size-check.XXXXXXX)
            savedhead=$(git rev-parse HEAD)
            # Push to PR base. First run of the script will write size files
            pr_base=$(git merge-base origin/$DEST_BRANCH HEAD)
            git checkout $pr_base
            hack/make-and-check-size $context_dir
            # pop back to PR, and run incremental makes. Subsequent script
            # invocations will compare against original size.
            git checkout $savedhead
            git rebase $pr_base -x "hack/make-and-check-size $context_dir"
            rm -rf $context_dir
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
    msg "podman info:"
    bin/podman info

    msg "Checking podman release (or potential release) criteria."
    # We're running under 'set -eo pipefail'; make sure this statement passes
    dev=$(bin/podman info |& grep -- -dev || echo -n '')
    if [[ -n "$dev" ]]; then
        die "Releases must never contain '-dev' in output of 'podman info' ($dev)"
    fi

    commit=$(bin/podman info --format='{{.Version.GitCommit}}' | tr -d '[:space:]')
    if [[ -z "$commit" ]]; then
        die "Releases must contain a non-empty Version.GitCommit in 'podman info'"
    fi
    msg "All OK"
}


# ***WARNING*** ***WARNING*** ***WARNING*** ***WARNING***
#    Please see gitlab comment in setup_environment.sh
# ***WARNING*** ***WARNING*** ***WARNING*** ***WARNING***
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

_run_machine() {
    # N/B: Can't use _bail_if_test_can_be_skipped here b/c content isn't under test/
    make localmachine |& logformatter
}

# Optimization: will exit if the only PR diffs are under docs/ or tests/
# with the exception of any given arguments. E.g., don't run e2e or upgrade
# or bud tests if the only PR changes are in test/system.
function _bail_if_test_can_be_skipped() {
    local head base diffs

    # Cirrus sets these for PRs but not branches or cron. In cron and branches,
    #we never want to skip.
    for v in CIRRUS_CHANGE_IN_REPO CIRRUS_PR DEST_BRANCH; do
        if [[ -z "${!v}" ]]; then
            msg "[ _cannot do selective skip: \$$v is undefined ]"
            return 0
        fi
    done
    # And if this one *is* defined, it means we're not in PR-land; don't skip.
    if [[ -n "$CIRRUS_TAG" ]]; then
        msg "[ _cannot do selective skip: \$CIRRUS_TAG is defined ]"
        return 0
    fi

    # Defined by Cirrus-CI for all tasks
    # shellcheck disable=SC2154
    head=$CIRRUS_CHANGE_IN_REPO
    base=$(git merge-base $DEST_BRANCH $head)
    diffs=$(git diff --name-only $base $head)

    # If PR touches any files in an argument directory, we cannot skip
    for subdir in "$@"; do
        if egrep -q "^$subdir/" <<<"$diffs"; then
            return 0
        fi
    done

    # PR does not touch any files under our input directories. Now see
    # if the PR touches files outside of the following directories, by
    # filtering these out from the diff results.
    for subdir in docs test; do
        # || true needed because we're running with set -e
        diffs=$(egrep -v "^$subdir/" <<<"$diffs" || true)
    done

    # If we still have diffs, they indicate files outside of docs & test.
    # It is not safe to skip.
    if [[ -n "$diffs" ]]; then
        return 0
    fi

    msg "SKIPPING: This is a doc- and/or test-only PR with no changes under $*"
    exit 0
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

    # This must be done at the last second, otherwise `make` calls
    # in setup_environment (as root) will balk about ownership.
    msg "Recursively chowning \$GOPATH and \$GOSRC to $ROOTLESS_USER"
    if [[ $PRIV_NAME = "rootless" ]]; then
        chown -R $ROOTLESS_USER:$ROOTLESS_USER "$GOPATH" "$GOSRC"
    fi

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
