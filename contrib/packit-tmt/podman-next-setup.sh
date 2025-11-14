#!/usr/bin/env bash

set -exo pipefail

COPR_REPO_FILE="/etc/yum.repos.d/_copr:copr.fedorainfracloud.org:rhcontainerbot:podman-next.repo"
if compgen -G "$COPR_REPO_FILE" > /dev/null; then
  # We want the priority bump appended to the file, we're not looking
  # to use a variable.
  # shellcheck disable=SC2016
  sed -i -n '/^priority=/!p;$apriority=1' "$COPR_REPO_FILE"
fi
# We want all dependencies from podman-next except podman as podman will be fetched
# from the packit copr.
dnf -y upgrade --allowerasing --exclude=podman*
