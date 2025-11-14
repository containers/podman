#!/usr/bin/env bash

set -exo pipefail

# Farm tag repo is set at higher priority, conflicting with podman-next
sed -i '/^priority/d' /etc/yum.repos.d/tag-repository.repo

# This should work even when podman-next isn't installed. It'll fetch the
# highest versions available across all repos.
dnf -y upgrade --allowerasing --exclude=podman*
