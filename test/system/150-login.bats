#!/usr/bin/env bats   -*- bats -*-
#
# tests for podman login
#

load helpers
load helpers.network
load helpers.registry

###############################################################################
# BEGIN filtering - none of these tests will work with podman-remote

function setup() {
    skip_if_remote "none of these tests work with podman-remote"

    basic_setup
    start_registry
}

# END   filtering - none of these tests will work with podman-remote
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
       "Error: logging into \"$registry\": invalid username/password" \
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
    is "$output" ".* checking whether a blob .* exists in localhost:${PODMAN_LOGIN_REGISTRY_PORT}/badpush: authentication required" \
       "auth error on push"
}

function _push_search_test() {
    # Preserve image ID for later comparison against push/pulled image
    run_podman inspect --format '{{.Id}}' $IMAGE
    iid=$output

    destname=ok-$(random_string 10 | tr A-Z a-z)-ok
    # Use command-line credentials
    run_podman push --tls-verify=$1 \
               --format docker \
               --cert-dir ${PODMAN_LOGIN_WORKDIR}/trusted-registry-cert-dir \
               --creds ${PODMAN_LOGIN_USER}:${PODMAN_LOGIN_PASS} \
               $IMAGE localhost:${PODMAN_LOGIN_REGISTRY_PORT}/$destname

    # Search a pushed image without --cert-dir should fail if --tls-verify=true
    run_podman $2 search --tls-verify=$1 \
               --format "table {{.Name}}" \
               --creds ${PODMAN_LOGIN_USER}:${PODMAN_LOGIN_PASS} \
               localhost:${PODMAN_LOGIN_REGISTRY_PORT}/$destname

    # Search a pushed image without --creds should fail
    run_podman 125 search --tls-verify=$1 \
               --format "table {{.Name}}" \
               --cert-dir ${PODMAN_LOGIN_WORKDIR}/trusted-registry-cert-dir \
               localhost:${PODMAN_LOGIN_REGISTRY_PORT}/$destname

    # Search a pushed image should succeed
    run_podman search --tls-verify=$1 \
               --format "table {{.Name}}" \
               --cert-dir ${PODMAN_LOGIN_WORKDIR}/trusted-registry-cert-dir \
               --creds ${PODMAN_LOGIN_USER}:${PODMAN_LOGIN_PASS} \
               localhost:${PODMAN_LOGIN_REGISTRY_PORT}/$destname
    is "${lines[1]}" "localhost:${PODMAN_LOGIN_REGISTRY_PORT}/$destname" "search output is destname"

    # Yay! Pull it back
    run_podman pull --tls-verify=$1 \
               --cert-dir ${PODMAN_LOGIN_WORKDIR}/trusted-registry-cert-dir \
               --creds ${PODMAN_LOGIN_USER}:${PODMAN_LOGIN_PASS} \
               localhost:${PODMAN_LOGIN_REGISTRY_PORT}/$destname

    # Compare to original image
    run_podman inspect --format '{{.Id}}' $destname
    is "$output" "$iid" "Image ID of pulled image == original IID"

    run_podman rmi $destname
}

@test "podman push and search ok with --tls-verify=false" {
    _push_search_test false 0
}

@test "podman push and search ok with --tls-verify=true" {
    _push_search_test true 125
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
    is "$output" ".*: authentication required" \
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

@test "podman manifest --tls-verify - basic test" {
    run_podman login --tls-verify=false \
               --username ${PODMAN_LOGIN_USER} \
               --password-stdin \
               localhost:${PODMAN_LOGIN_REGISTRY_PORT} <<<"${PODMAN_LOGIN_PASS}"
    is "$output" "Login Succeeded!" "output from podman login"

    manifest1="localhost:${PODMAN_LOGIN_REGISTRY_PORT}/test:1.0"
    run_podman manifest create $manifest1
    mid=$output
    run_podman manifest push --authfile=$authfile \
        --tls-verify=false $mid \
        $manifest1
    run_podman manifest rm $manifest1
    run_podman manifest inspect --insecure $manifest1
    is "$output" ".*\"mediaType\": \"application/vnd.docker.distribution.manifest.list.v2+json\"" "Verify --insecure works against an insecure registry"
    run_podman 125 manifest inspect --insecure=false $manifest1
    is "$output" ".*Error: reading image \"docker://$manifest1\": pinging container registry localhost:${PODMAN_LOGIN_REGISTRY_PORT}:" "Verify --insecure=false fails"
    run_podman manifest inspect --tls-verify=false $manifest1
    is "$output" ".*\"mediaType\": \"application/vnd.docker.distribution.manifest.list.v2+json\"" "Verify --tls-verify=false works against an insecure registry"
    run_podman 125 manifest inspect --tls-verify=true $manifest1
    is "$output" ".*Error: reading image \"docker://$manifest1\": pinging container registry localhost:${PODMAN_LOGIN_REGISTRY_PORT}:" "Verify --tls-verify=true fails"

    # Now log out
    run_podman logout localhost:${PODMAN_LOGIN_REGISTRY_PORT}
    is "$output" "Removed login credentials for localhost:${PODMAN_LOGIN_REGISTRY_PORT}" \
       "output from podman logout"
}

# END   cooperation with skopeo
# END   actual tests
###############################################################################

# vim: filetype=sh
