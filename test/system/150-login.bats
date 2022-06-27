#!/usr/bin/env bats   -*- bats -*-
#
# tests for podman login
#

load helpers

###############################################################################
# BEGIN one-time envariable setup

# Create a scratch directory; our podman registry will run from here. We
# also use it for other temporary files like authfiles.
if [ -z "${PODMAN_LOGIN_WORKDIR}" ]; then
    export PODMAN_LOGIN_WORKDIR=$(mktemp -d --tmpdir=${BATS_TMPDIR:-${TMPDIR:-/tmp}} podman_bats_login.XXXXXX)
fi

# Randomly-generated username and password
if [ -z "${PODMAN_LOGIN_USER}" ]; then
    export PODMAN_LOGIN_USER="user$(random_string 4)"
    export PODMAN_LOGIN_PASS=$(random_string 15)
fi

# Randomly-assigned port in the 5xxx range
if [ -z "${PODMAN_LOGIN_REGISTRY_PORT}" ]; then
    export PODMAN_LOGIN_REGISTRY_PORT=$(random_free_port)
fi

# Override any user-set path to an auth file
unset REGISTRY_AUTH_FILE

# END   one-time envariable setup
###############################################################################
# BEGIN filtering - none of these tests will work with podman-remote

function setup() {
    skip_if_remote "none of these tests work with podman-remote"

    basic_setup
}

# END   filtering - none of these tests will work with podman-remote
###############################################################################
# BEGIN first "test" - start a registry for use by other tests
#
# This isn't really a test: it's a helper that starts a local registry.
# Note that we're careful to use a root/runroot separate from our tests,
# so setup/teardown don't clobber our registry image.
#

@test "podman login [start registry]" {
    AUTHDIR=${PODMAN_LOGIN_WORKDIR}/auth
    mkdir -p $AUTHDIR

    # Registry image; copy of docker.io, but on our own registry
    local REGISTRY_IMAGE="$PODMAN_TEST_IMAGE_REGISTRY/$PODMAN_TEST_IMAGE_USER/registry:2.7"

    # Pull registry image, but into a separate container storage
    mkdir -p ${PODMAN_LOGIN_WORKDIR}/root
    mkdir -p ${PODMAN_LOGIN_WORKDIR}/runroot
    PODMAN_LOGIN_ARGS="--storage-driver=vfs --root ${PODMAN_LOGIN_WORKDIR}/root --runroot ${PODMAN_LOGIN_WORKDIR}/runroot"
    # Give it three tries, to compensate for flakes
    run_podman ${PODMAN_LOGIN_ARGS} pull $REGISTRY_IMAGE ||
        run_podman ${PODMAN_LOGIN_ARGS} pull $REGISTRY_IMAGE ||
        run_podman ${PODMAN_LOGIN_ARGS} pull $REGISTRY_IMAGE

    # Registry image needs a cert. Self-signed is good enough.
    CERT=$AUTHDIR/domain.crt
    if [ ! -e $CERT ]; then
        openssl req -newkey rsa:4096 -nodes -sha256 \
                -keyout $AUTHDIR/domain.key -x509 -days 2 \
                -out $AUTHDIR/domain.crt \
                -subj "/C=US/ST=Foo/L=Bar/O=Red Hat, Inc./CN=localhost"
    fi

    # Store credentials where container will see them
    if [ ! -e $AUTHDIR/htpasswd ]; then
        htpasswd -Bbn ${PODMAN_LOGIN_USER} ${PODMAN_LOGIN_PASS} \
                 > $AUTHDIR/htpasswd

        # In case $PODMAN_TEST_KEEP_LOGIN_REGISTRY is set, for testing later
        echo "${PODMAN_LOGIN_USER}:${PODMAN_LOGIN_PASS}" \
             > $AUTHDIR/htpasswd-plaintext
    fi

    # Run the registry container.
    run_podman '?' ${PODMAN_LOGIN_ARGS} rm -t 0 -f registry
    run_podman ${PODMAN_LOGIN_ARGS} run -d \
               -p ${PODMAN_LOGIN_REGISTRY_PORT}:5000 \
               --name registry \
               -v $AUTHDIR:/auth:Z \
               -e "REGISTRY_AUTH=htpasswd" \
               -e "REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm" \
               -e REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd \
               -e REGISTRY_HTTP_TLS_CERTIFICATE=/auth/domain.crt \
               -e REGISTRY_HTTP_TLS_KEY=/auth/domain.key \
               $REGISTRY_IMAGE
}

# END   first "test" - start a registry for use by other tests
###############################################################################
# BEGIN actual tests
# BEGIN primary podman login/push/pull tests

@test "podman login - basic test" {
    run_podman login --tls-verify=false \
               --username ${PODMAN_LOGIN_USER} \
               --password-stdin \
               localhost:${PODMAN_LOGIN_REGISTRY_PORT} <<<"${PODMAN_LOGIN_PASS}"
    is "$output" "Login Succeeded!" "output from podman login"

    # Now log out
    run_podman logout localhost:${PODMAN_LOGIN_REGISTRY_PORT}
    is "$output" "Removed login credentials for localhost:${PODMAN_LOGIN_REGISTRY_PORT}" \
       "output from podman logout"
}

@test "podman login - with wrong credentials" {
    registry=localhost:${PODMAN_LOGIN_REGISTRY_PORT}

    run_podman 125 login --tls-verify=false \
               --username ${PODMAN_LOGIN_USER} \
               --password-stdin \
               $registry <<< "x${PODMAN_LOGIN_PASS}"
    is "$output" \
       "Error: error logging into \"$registry\": invalid username/password" \
       'output from podman login'
}

@test "podman login - check generated authfile" {
    authfile=${PODMAN_LOGIN_WORKDIR}/auth-$(random_string 10).json
    rm -f $authfile

    registry=localhost:${PODMAN_LOGIN_REGISTRY_PORT}

    run_podman login --authfile=$authfile \
        --tls-verify=false \
        --username ${PODMAN_LOGIN_USER} \
        --password ${PODMAN_LOGIN_PASS} \
        $registry

    # Confirm that authfile now exists
    test -e $authfile || \
        die "podman login did not create authfile $authfile"

    # Special bracket form needed because of colon in host:port
    run jq -r ".[\"auths\"][\"$registry\"][\"auth\"]" <$authfile
    is "$status" "0" "jq from $authfile"

    expect_userpass="${PODMAN_LOGIN_USER}:${PODMAN_LOGIN_PASS}"
    actual_userpass=$(base64 -d <<<"$output")
    is "$actual_userpass" "$expect_userpass" "credentials stored in $authfile"


    # Now log out and make sure credentials are removed
    run_podman logout --authfile=$authfile $registry

    run jq -r '.auths' <$authfile
    is "$status" "0" "jq from $authfile"
    is "$output" "{}" "credentials removed from $authfile"
}

# Some push tests
@test "podman push fail" {

    # Create an invalid authfile
    authfile=${PODMAN_LOGIN_WORKDIR}/auth-$(random_string 10).json
    rm -f $authfile

    wrong_auth=$(base64 <<<"baduser:wrongpassword")
    cat >$authfile <<EOF
{
    "auths": {
            "localhost:${PODMAN_LOGIN_REGISTRY_PORT}": {
                    "auth": "$wrong_auth"
            }
    }
}
EOF

    run_podman 125 push --authfile=$authfile \
        --tls-verify=false $IMAGE \
        localhost:${PODMAN_LOGIN_REGISTRY_PORT}/badpush:1
    is "$output" ".*: unauthorized: authentication required" \
       "auth error on push"
}

@test "podman push ok" {
    # Preserve image ID for later comparison against push/pulled image
    run_podman inspect --format '{{.Id}}' $IMAGE
    iid=$output

    destname=ok-$(random_string 10 | tr A-Z a-z)-ok
    # Use command-line credentials
    run_podman push --tls-verify=false \
               --format docker \
               --creds ${PODMAN_LOGIN_USER}:${PODMAN_LOGIN_PASS} \
               $IMAGE localhost:${PODMAN_LOGIN_REGISTRY_PORT}/$destname

    # Yay! Pull it back
    run_podman pull --tls-verify=false \
               --creds ${PODMAN_LOGIN_USER}:${PODMAN_LOGIN_PASS} \
               localhost:${PODMAN_LOGIN_REGISTRY_PORT}/$destname

    # Compare to original image
    run_podman inspect --format '{{.Id}}' $destname
    is "$output" "$iid" "Image ID of pulled image == original IID"

    run_podman rmi $destname
}

# END   primary podman login/push/pull tests
###############################################################################
# BEGIN cooperation with skopeo

# Skopeo helper - keep this separate, so we can test with different
# envariable settings
function _test_skopeo_credential_sharing() {
    if ! type -p skopeo; then
        skip "skopeo not available"
    fi

    registry=localhost:${PODMAN_LOGIN_REGISTRY_PORT}

    run_podman login "$@" --tls-verify=false \
               --username ${PODMAN_LOGIN_USER} \
               --password ${PODMAN_LOGIN_PASS} \
               $registry

    destname=skopeo-ok-$(random_string 10 | tr A-Z a-z)-ok
    echo "# skopeo copy ..."
    run skopeo copy "$@" \
        --format=v2s2 \
        --dest-tls-verify=false \
        containers-storage:$IMAGE \
        docker://$registry/$destname
    echo "$output"
    is "$status" "0" "skopeo copy - exit status"
    is "$output" ".*Copying blob .*"     "output of skopeo copy"
    is "$output" ".*Copying config .*"   "output of skopeo copy"
    is "$output" ".*Writing manifest .*" "output of skopeo copy"

    echo "# skopeo inspect ..."
    run skopeo inspect "$@" --tls-verify=false docker://$registry/$destname
    echo "$output"
    is "$status" "0" "skopeo inspect - exit status"

    got_name=$(jq -r .Name <<<"$output")
    is "$got_name" "$registry/$destname" "skopeo inspect -> Name"

    # Now try without a valid login; it should fail
    run_podman logout "$@" $registry
    echo "# skopeo inspect [with no credentials] ..."
    run skopeo inspect "$@" --tls-verify=false docker://$registry/$destname
    echo "$output"
    is "$status" "1" "skopeo inspect - exit status"
    is "$output" ".*: unauthorized: authentication required" \
       "auth error on skopeo inspect"
}

@test "podman login - shares credentials with skopeo - default auth file" {
    if is_rootless; then
        if [ -z "${XDG_RUNTIME_DIR}" ]; then
            skip "skopeo does not match podman when XDG_RUNTIME_DIR unset; #823"
        fi
    fi
    _test_skopeo_credential_sharing
}

@test "podman login - shares credentials with skopeo - via envariable" {
    authfile=${PODMAN_LOGIN_WORKDIR}/auth-$(random_string 10).json
    rm -f $authfile

    REGISTRY_AUTH_FILE=$authfile _test_skopeo_credential_sharing
    rm -f $authfile
}

@test "podman login - shares credentials with skopeo - via --authfile" {
    # Also test that command-line --authfile overrides envariable
    authfile=${PODMAN_LOGIN_WORKDIR}/auth-$(random_string 10).json
    rm -f $authfile

    fake_authfile=${PODMAN_LOGIN_WORKDIR}/auth-$(random_string 10).json
    rm -f $fake_authfile

    REGISTRY_AUTH_FILE=$authfile _test_skopeo_credential_sharing --authfile=$authfile

    if [ -e $fake_authfile ]; then
        die "REGISTRY_AUTH_FILE overrode command-line --authfile!"
    fi
    rm -f $authfile
}

# END   cooperation with skopeo
# END   actual tests
###############################################################################
# BEGIN teardown (remove the registry container)

@test "podman login [stop registry, clean up]" {
    # For manual debugging; user may request keeping the registry running
    if [ -n "${PODMAN_TEST_KEEP_LOGIN_REGISTRY}" ]; then
        skip "[leaving registry running by request]"
    fi

    run_podman --storage-driver=vfs --root    ${PODMAN_LOGIN_WORKDIR}/root   \
               --runroot ${PODMAN_LOGIN_WORKDIR}/runroot \
               rm -f registry
    run_podman --storage-driver=vfs --root    ${PODMAN_LOGIN_WORKDIR}/root   \
               --runroot ${PODMAN_LOGIN_WORKDIR}/runroot \
               rmi -a

    # By default, clean up
    if [ -z "${PODMAN_TEST_KEEP_LOGIN_WORKDIR}" ]; then
        rm -rf ${PODMAN_LOGIN_WORKDIR}
    fi

    # Make sure socket is closed
    if ! port_is_free $PODMAN_LOGIN_REGISTRY_PORT; then
        die "Socket still seems open"
    fi
}

# END   teardown (remove the registry container)
###############################################################################

# vim: filetype=sh
