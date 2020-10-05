#!/bin/bash

set -eo pipefail

# This script is intended to be called by automation or humans,
# from a specially configured environment.  Depending on the contents
# of various variable, entirely different operations will be performed.

# shellcheck source=contrib/cirrus/lib.sh
source $(dirname $0)/lib.sh

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

build_swagger() {
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
}

altbuild() {
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

    # 'logformatter' script makes test logs readable; only works for some tests
    case "$testsuite" in
        integration|system)  output_filter=logformatter ;;
        *)                   output_filter="cat"        ;;
    esac

    # containers/automation sets this to 0 for it's dbg() function
    # but the e2e integration tests are also sensitive to it.
    unset DEBUG

    # shellcheck disable=SC2154
    case "$PODBIN_NAME" in
        podman)
            # ginkgo doesn't play nicely with C Go
            make local${testsuite} \
                |& "$output_filter"
            ;;
        remote)
            make remote${testsuite} PODMAN_SERVER_LOG=$PODMAN_SERVER_LOG \
                |& "$output_filter"
            ;;
    esac
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

case "$TEST_FLAVOR" in
    ext_svc) $SCRIPT_BASE/ext_svc_check.sh ;;
    smoke)
        make gofmt
        # There is little value to validating commits after tag-push
        # and it's very difficult to automatically determine a starting commit.
        # $CIRRUS_TAG is only non-empty when executing due to a tag-push
        # shellcheck disable=SC2154
        if [[ -z "$CIRRUS_TAG" ]]; then
            make .gitvalidation
        fi
        ;;
    automation)
        $SCRIPT_BASE/cirrus_yaml_test.py
        req_env_vars CI DEST_BRANCH IMAGE_SUFFIX TEST_FLAVOR TEST_ENVIRON \
                     PODBIN_NAME PRIV_NAME DISTRO_NV CONTAINER USER HOME \
                     UID GID AUTOMATION_LIB_PATH SCRIPT_BASE OS_RELEASE_ID \
                     OS_RELEASE_VER CG_FS_TYPE
        bigto ooe.sh dnf install -y ShellCheck  # small/quick addition
        $SCRIPT_BASE/shellcheck.sh
        ;;
    altbuild) altbuild ;;
    build)
        make podman-release
        make podman-remote-linux-release
        ;;
    validate)
        # Confirm compiile via prior task + cache
        bin/podman --version
        bin/podman-remote --version
        make validate  # Some items require a build
        ;;
    bindings)
        # shellcheck disable=SC2155
        export PATH=$PATH:$GOSRC/hack
        # Subshell needed for .cirrus.yml to find logformatter output in cwd
        (cd pkg/bindings/test && ginkgo -trace -noColor -debug  -r) |& logformatter
        ;;
    endpoint)
        make test-binaries
        make endpoint
        ;;
    swagger)
        build_swagger
        # Cirrus-CI Artifact instruction expects file here
        cp -v $GOSRC/pkg/api/swagger.yaml $GOSRC/
        ;;
    vendor)
        make vendor
        ./hack/tree_status.sh
        ;;
    docker-py) msg "This is docker-py stub, it is only a stub" ;;
    unit) make localunit ;;
    int) dotest integration ;;
    sys) dotest system ;;
    release)
        if bin/podman info |& grep -Eq -- '-dev'; then
            die "Releases must never contain '-dev' in output of 'podman info'"
        fi
        ;;
    *)
        die "Unknown/Unsupported \$TEST_FLAVOR=$TEST_FLAVOR" ;;
esac
