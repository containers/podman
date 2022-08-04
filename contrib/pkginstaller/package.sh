#!/bin/bash

set -euxo pipefail

BASEDIR=$(dirname "$0")
OUTPUT=$1
CODESIGN_IDENTITY=${CODESIGN_IDENTITY:-mock}
PRODUCTSIGN_IDENTITY=${PRODUCTSIGN_IDENTITY:-mock}
NO_CODESIGN=${NO_CODESIGN:-0}
HELPER_BINARIES_DIR="/opt/podman/qemu/bin"

binDir="${BASEDIR}/root/podman/bin"

function build_podman() {
  pushd "$1"
    make podman-remote HELPER_BINARIES_DIR="${HELPER_BINARIES_DIR}"
    make podman-mac-helper
    cp bin/darwin/podman "contrib/pkginstaller/out/packaging/${binDir}/podman"
    cp bin/darwin/podman-mac-helper "contrib/pkginstaller/out/packaging/${binDir}/podman-mac-helper"
  popd
}

function sign() {
  if [ "${NO_CODESIGN}" -eq "1" ]; then
    return
  fi
  local opts=""
  entitlements="${BASEDIR}/$(basename "$1").entitlements"
  if [ -f "${entitlements}" ]; then
      opts="--entitlements ${entitlements}"
  fi
  codesign --deep --sign "${CODESIGN_IDENTITY}" --options runtime --force --timestamp "${opts}" "$1"
}

version=$(cat "${BASEDIR}/VERSION")
arch=$(cat "${BASEDIR}/ARCH")

build_podman "../../../../"
sign "${binDir}/podman"
sign "${binDir}/gvproxy"
sign "${binDir}/podman-mac-helper"

pkgbuild --identifier com.redhat.podman --version "${version}" \
  --scripts "${BASEDIR}/scripts" \
  --root "${BASEDIR}/root" \
  --install-location /opt \
  --component-plist "${BASEDIR}/component.plist" \
  "${OUTPUT}/podman.pkg"

productbuild --distribution "${BASEDIR}/Distribution" \
  --resources "${BASEDIR}/Resources" \
  --package-path "${OUTPUT}" \
  "${OUTPUT}/podman-unsigned.pkg"
rm "${OUTPUT}/podman.pkg"

if [ ! "${NO_CODESIGN}" -eq "1" ]; then
  productsign --timestamp --sign "${PRODUCTSIGN_IDENTITY}" "${OUTPUT}/podman-unsigned.pkg" "${OUTPUT}/podman-installer-macos-${arch}.pkg"
else
  mv "${OUTPUT}/podman-unsigned.pkg" "${OUTPUT}/podman-installer-macos-${arch}.pkg"
fi
