#!/usr/bin/env bash

# This script generates release RPMs into _output/releases.
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::util::ensure::system_binary_exists rpmbuild
os::util::ensure::system_binary_exists createrepo
os::build::rpm::get_nvra_vars
OS_RPM_NAME="podman"

# create the rpm
make rpm -f ${OS_ROOT}/.copr/Makefile

# migrate the rpm artifacts to the output directory, must be clean or move will fail
make clean
mkdir -p "${OS_OUTPUT}"

mkdir -p "${OS_OUTPUT_RPMPATH}"
mv -f "${OS_ROOT}"/x86_64/*.rpm "${OS_OUTPUT_RPMPATH}"

mkdir -p "${OS_OUTPUT_RELEASEPATH}"
echo "${OS_GIT_COMMIT}" > "${OS_OUTPUT_RELEASEPATH}/.commit"

repo_path=${OS_OUTPUT_RPMPATH}
createrepo "${repo_path}"

echo "[${OS_RPM_NAME}-local-release]
baseurl = file://${repo_path}
gpgcheck = 0
name = Release from Local Source for ${OS_RPM_NAME}
enabled = 1
" > "${repo_path}/local-release.repo"

# DEPRECATED: preserve until jobs migrate to using local-release.repo
cp "${repo_path}/local-release.repo" "${repo_path}/podman-local-release.repo"

os::log::info "Repository file for \`yum\` or \`dnf\` placed at ${repo_path}/local-release.repo
Install it with:
$ mv '${repo_path}/local-release.repo' '/etc/yum.repos.d"