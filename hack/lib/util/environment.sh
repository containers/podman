#!/usr/bin/env bash

# This script holds library functions for setting up the shell environment for OpenShift scripts

# os::util::environment::update_path_var updates $PATH so that OpenShift binaries are available
#
# Globals:
#  - OS_ROOT
#  - PATH
# Arguments:
#  None
# Returns:
#  - export PATH
function os::util::environment::update_path_var() {
    local prefix
    if os::util::find::system_binary 'go' >/dev/null 2>&1; then
        prefix+="${OS_OUTPUT_BINPATH}/$(os::build::host_platform):"
    fi
    if [[ -n "${GOPATH:-}" ]]; then
        prefix+="${GOPATH}/bin:"
    fi

    PATH="${prefix:-}${PATH}"
    export PATH
}
readonly -f os::util::environment::update_path_var

# os::util::environment::setup_tmpdir_vars sets up temporary directory path variables
#
# Globals:
#  - TMPDIR
# Arguments:
#  - 1: the path under the root temporary directory for OpenShift where these subdirectories should be made
# Returns:
#  - export BASETMPDIR
#  - export BASEOUTDIR
#  - export LOG_DIR
#  - export VOLUME_DIR
#  - export ARTIFACT_DIR
#  - export FAKE_HOME_DIR
#  - export OS_TMP_ENV_SET
function os::util::environment::setup_tmpdir_vars() {
    local sub_dir=$1

    BASETMPDIR="${TMPDIR:-/tmp}/podman/${sub_dir}"
    export BASETMPDIR
    VOLUME_DIR="${BASETMPDIR}/volumes"
    export VOLUME_DIR

    BASEOUTDIR="${OS_OUTPUT_SCRIPTPATH}/${sub_dir}"
    export BASEOUTDIR
    LOG_DIR="${ARTIFACT_DIR:-${BASEOUTDIR}}/logs"
    export LOG_DIR
    ARTIFACT_DIR="${ARTIFACT_DIR:-${BASEOUTDIR}/artifacts}"
    export ARTIFACT_DIR
    FAKE_HOME_DIR="${BASEOUTDIR}/podman.local.home"
    export FAKE_HOME_DIR

    mkdir -p "${LOG_DIR}" "${VOLUME_DIR}" "${ARTIFACT_DIR}" "${FAKE_HOME_DIR}"

    export OS_TMP_ENV_SET="${sub_dir}"
}
readonly -f os::util::environment::setup_tmpdir_vars