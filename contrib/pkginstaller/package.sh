#!/bin/bash

set -euxo pipefail

BASEDIR=$(dirname "$0")
OUTPUT=$1
CODESIGN_IDENTITY=${CODESIGN_IDENTITY:-mock}
PRODUCTSIGN_IDENTITY=${PRODUCTSIGN_IDENTITY:-mock}
NO_CODESIGN=${NO_CODESIGN:-0}
HELPER_BINARIES_DIR="/opt/podman/qemu/bin"

binDir="${BASEDIR}/root/podman/bin"
qemuBinDir="${BASEDIR}/root/podman/qemu/bin"

version=$(cat "${BASEDIR}/VERSION")
arch=$(cat "${BASEDIR}/ARCH")

function build_podman() {
  pushd "$1"
    make GOARCH="${goArch}" podman-remote HELPER_BINARIES_DIR="${HELPER_BINARIES_DIR}"
    make GOARCH="${goArch}" podman-mac-helper
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
  codesign --deep --sign "${CODESIGN_IDENTITY}" --options runtime --timestamp --force ${opts} "$1"
}

function signQemu() {
  if [ "${NO_CODESIGN}" -eq "1" ]; then
    return
  fi

  local qemuArch="${arch}"
  if [ "${qemuArch}" = amd64 ]; then
      qemuArch=x86_64
  fi

  # sign the files inside /opt/podman/qemu/lib
  libs=$(find "${BASEDIR}"/root/podman/qemu/lib -depth -name "*.dylib" -or -type f -perm +111)
  echo "${libs}" | xargs -t -I % codesign --deep --sign "${CODESIGN_IDENTITY}" --options runtime --timestamp --force % || true

  # sign the files inside /opt/podman/qemu/bin except qemu-system-*
  bins=$(find "${BASEDIR}"/root/podman/qemu/bin -depth -type f -perm +111 ! -name "qemu-system-${qemuArch}")
  echo "${bins}" | xargs -t -I % codesign --deep --sign "${CODESIGN_IDENTITY}" --options runtime --timestamp --force  % || true

  # sign the qemu-system-* binary
  # need to remove any extended attributes, otherwise codesign complains:
  # qemu-system-aarch64: resource fork, Finder information, or similar detritus not allowed
  xattr -cr "${qemuBinDir}/qemu-system-${qemuArch}"
  codesign --deep --sign "${CODESIGN_IDENTITY}" --options runtime --timestamp --force \
    --entitlements "${BASEDIR}/hvf.entitlements" "${qemuBinDir}/qemu-system-${qemuArch}"
}

goArch="${arch}"
if [ "${goArch}" = aarch64 ]; then
  goArch=arm64
fi

build_podman "../../../../"
sign "${binDir}/podman"
sign "${binDir}/gvproxy"
sign "${binDir}/podman-mac-helper"
signQemu

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
  productsign --timestamp --sign "${PRODUCTSIGN_IDENTITY}" "${OUTPUT}/podman-unsigned.pkg" "${OUTPUT}/podman-installer-macos-${goArch}.pkg"
else
  mv "${OUTPUT}/podman-unsigned.pkg" "${OUTPUT}/podman-installer-macos-${goArch}.pkg"
fi
