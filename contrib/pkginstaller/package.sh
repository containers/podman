#!/bin/bash

set -euxo pipefail

BASEDIR=$(dirname "$0")
OUTPUT=$1
CODESIGN_IDENTITY=${CODESIGN_IDENTITY:--}
PRODUCTSIGN_IDENTITY=${PRODUCTSIGN_IDENTITY:-mock}
NO_CODESIGN=${NO_CODESIGN:-0}
HELPER_BINARIES_DIR="/opt/podman/bin"
MACHINE_POLICY_JSON_DIR="/opt/podman/config"

tmpBin="contrib/pkginstaller/tmp-bin"

binDir="${BASEDIR}/root/podman/bin"

version=$(cat "${BASEDIR}/VERSION")
arch=$(cat "${BASEDIR}/ARCH")

function build_podman() {
  pushd "$1"

  case ${goArch} in
  universal)
    build_fat
    cp "${tmpBin}/podman-universal"  "contrib/pkginstaller/out/packaging/${binDir}/podman"
    cp "${tmpBin}/podman-mac-helper-universal" "contrib/pkginstaller/out/packaging/${binDir}/podman-mac-helper"
    ;;

  amd64 | arm64)
    build_podman_arch ${goArch}
    cp "${tmpBin}/podman-${goArch}"  "contrib/pkginstaller/out/packaging/${binDir}/podman"
    cp "${tmpBin}/podman-mac-helper-${goArch}" "contrib/pkginstaller/out/packaging/${binDir}/podman-mac-helper"
    ;;
  *)
    echo -n "Unknown arch: ${goArch}"
    ;;
  esac

  popd
}

function build_podman_arch(){
    make -B GOARCH="$1" podman-remote HELPER_BINARIES_DIR="${HELPER_BINARIES_DIR}"
    make -B GOARCH="$1" podman-mac-helper
    mkdir -p "${tmpBin}"
    cp bin/darwin/podman "${tmpBin}/podman-$1"
    cp bin/darwin/podman-mac-helper "${tmpBin}/podman-mac-helper-$1"
}

function build_fat(){
    echo "Building ARM Podman"
    build_podman_arch "arm64"
    echo "Building AMD Podman"
    build_podman_arch "amd64"

    echo "Creating universal binary"
    lipo -create -output "${tmpBin}/podman-universal" "${tmpBin}/podman-arm64" "${tmpBin}/podman-amd64"
    lipo -create -output "${tmpBin}/podman-mac-helper-universal" "${tmpBin}/podman-mac-helper-arm64" "${tmpBin}/podman-mac-helper-amd64"
}

function sign() {
  local opts=""
  entitlements="${BASEDIR}/$(basename "$1").entitlements"
  if [ -f "${entitlements}" ]; then
      opts="--entitlements ${entitlements}"
  fi
  codesign --deep --sign "${CODESIGN_IDENTITY}" --options runtime --timestamp --force ${opts} "$1"
}

goArch="${arch}"
if [ "${goArch}" = aarch64 ]; then
  goArch=arm64
fi

build_podman "../../../../"

sign "${binDir}/podman"
sign "${binDir}/gvproxy"
sign "${binDir}/vfkit"
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
  productsign --timestamp --sign "${PRODUCTSIGN_IDENTITY}" "${OUTPUT}/podman-unsigned.pkg" "${OUTPUT}/podman-installer-macos-${goArch}.pkg"
else
  mv "${OUTPUT}/podman-unsigned.pkg" "${OUTPUT}/podman-installer-macos-${goArch}.pkg"
fi
