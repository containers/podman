#!/usr/bin/env bash

# This library holds utility functions for building
# and placing Golang binaries for multiple arches.

# Asks golang what it thinks the host platform is.  The go tool chain does some
# slightly different things when the target platform matches the host platform.
function os::build::host_platform() {
  echo "$(go env GOHOSTOS)/$(go env GOHOSTARCH)"
}
readonly -f os::build::host_platform

# os::build::setup_env will check that the `go` commands is available in
# ${PATH}. If not running on Travis, it will also check that the Go version is
# good enough for the Kubernetes build.
#
# Output Vars:
#   export GOPATH - A modified GOPATH to our created tree along with extra
#     stuff.
#   export GOBIN - This is actively unset if already set as we want binaries
#     placed in a predictable place.
function os::build::setup_env() {
  os::util::ensure::system_binary_exists 'go'

  if [[ -z "$(which sha256sum)" ]]; then
    sha256sum() {
      return 0
    }
  fi

  # Travis continuous build uses a head go release that doesn't report
  # a version number, so we skip this check on Travis.  It's unnecessary
  # there anyway.
  if [[ "${TRAVIS:-}" != "true" ]]; then
    os::golang::verify_go_version
  fi
  # For any tools that expect this to be set (it is default in golang 1.6),
  # force vendor experiment.
  export GO15VENDOREXPERIMENT=1

  unset GOBIN

  # create a local GOPATH in _output
  GOPATH="${OS_OUTPUT}/go"
  OS_TARGET_BIN="${OS_OUTPUT}/go/bin"
  local go_pkg_dir="${GOPATH}/src/${OS_GO_PACKAGE}"
  local go_pkg_basedir=$(dirname "${go_pkg_dir}")

  mkdir -p "${go_pkg_basedir}"
  rm -f "${go_pkg_dir}"

  # TODO: This symlink should be relative.
  ln -s "${OS_ROOT}" "${go_pkg_dir}"

  # lots of tools "just don't work" unless we're in the GOPATH
  cd "${go_pkg_dir}"

  # Append OS_EXTRA_GOPATH to the GOPATH if it is defined.
  if [[ -n ${OS_EXTRA_GOPATH:-} ]]; then
    GOPATH="${GOPATH}:${OS_EXTRA_GOPATH}"
  fi

  export GOPATH
  export OS_TARGET_BIN
}
readonly -f os::build::setup_env