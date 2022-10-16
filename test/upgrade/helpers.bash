# -*- bash -*-

load "../system/helpers"
load "../system/helpers.network"	# For random_free_port()

setup() {
    :
}

teardown() {
    :
}

# skip a test when the given version is older than the currently tested one
skip_if_version_older() {
    # use ${PODMAN_UPGRADE_FROM##v} to trim the leading "v"
    if printf '%s\n%s\n' "${PODMAN_UPGRADE_FROM##v}" "$1" | sort --check=quiet --version-sort; then
        skip "${2-test is only meaningful when upgrading from $1 or later}"
    fi
}
