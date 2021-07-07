#!/usr/bin/env bats

load helpers

function setup() {
    skip_if_remote "--sign-by does not work with podman-remote"

    basic_setup

    export _GNUPGHOME_TMP=$PODMAN_TMPDIR/.gnupg
    mkdir --mode=0700 $_GNUPGHOME_TMP $PODMAN_TMPDIR/signatures

    cat >$PODMAN_TMPDIR/keydetails <<EOF
    %echo Generating a basic OpenPGP key
    Key-Type: RSA
    Key-Length: 2048
    Subkey-Type: RSA
    Subkey-Length: 2048
    Name-Real: Foo
    Name-Comment: Foo
    Name-Email: foo@bar.com
    Expire-Date: 0
    %no-ask-passphrase
    %no-protection
    # Do a commit here, so that we can later print "done" :-)
    %commit
    %echo done
EOF
    GNUPGHOME=$_GNUPGHOME_TMP gpg --verbose --batch --gen-key $PODMAN_TMPDIR/keydetails
}

function check_signature() {
    ls -laR $PODMAN_TMPDIR/signatures
    run_podman inspect --format '{{.Digest}}' $PODMAN_TEST_IMAGE_FQN
    local repodigest=${output/:/=}

    local dir="$PODMAN_TMPDIR/signatures/libpod/${PODMAN_TEST_IMAGE_NAME}@${repodigest}"

    test -d $dir || die "Missing signature directory $dir"
    test -e "$dir/$1" || die "Missing signature file '$1'"

    # Confirm good signature
    GNUPGHOME=$_GNUPGHOME_TMP gpg --verify "$dir/$1"
}


@test "podman image - sign with no authfile" {
    GNUPGHOME=$_GNUPGHOME_TMP run_podman image sign --sign-by foo@bar.com --directory $PODMAN_TMPDIR/signatures  "docker://$PODMAN_TEST_IMAGE_FQN"
    check_signature "signature-1"
}

@test "podman image - sign with authfile" {
    local signature_file="$(random_string 10 | tr A-Z a-z)"

    GNUPGHOME=$_GNUPGHOME_TMP run_podman image sign --sign-by foo@bar.com --directory $PODMAN_TMPDIR/signatures  --authfile $signature_file "docker://$PODMAN_TEST_IMAGE_FQN"
    check_signature "$signature_file"
}

# vim: filetype=sh
