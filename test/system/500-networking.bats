#!/usr/bin/env bats   -*- bats -*-
#
# Test podman local networking
#

load helpers

# Copied from tsweeney's https://github.com/containers/libpod/issues/4827
@test "podman networking: port on localhost" {
    skip_if_remote
    random_1=$(random_string 30)
    random_2=$(random_string 30)

    HOST_PORT=8080
    SERVER=http://localhost:$HOST_PORT

    # Create a test file with random content
    INDEX1=$PODMAN_TMPDIR/hello.txt
    echo $random_1 > $INDEX1

    # Bind-mount this file with a different name to a container running httpd
    run_podman run -d --name myweb -p "$HOST_PORT:80" \
               -v $INDEX1:/var/www/index.txt \
               -w /var/www \
               busybox httpd -f -p 80
    cid=$output

    # In that container, create a second file, using exec and redirection
    run_podman exec -i myweb sh -c "cat > index2.txt" <<<"$random_2"
    # ...verify its contents as seen from container.
    run_podman exec -i myweb cat /var/www/index2.txt
    is "$output" "$random_2" "exec cat index2.txt"

    # Verify http contents: curl from localhost
    run curl -s $SERVER/index.txt
    is "$output" "$random_1" "curl localhost:/index.txt"
    run curl -s $SERVER/index2.txt
    is "$output" "$random_2" "curl localhost:/index2.txt"

    # Verify http contents: wget from a second container
    run_podman run --rm --net=host busybox wget -qO - $SERVER/index.txt
    is "$output" "$random_1" "podman wget /index.txt"
    run_podman run --rm --net=host busybox wget -qO - $SERVER/index2.txt
    is "$output" "$random_2" "podman wget /index2.txt"

    # Tests #4889 - two-argument form of "podman ports" was broken
    run_podman port myweb
    is "$output" "80/tcp -> 0.0.0.0:$HOST_PORT" "port <cid>"
    run_podman port myweb 80
    is "$output" "0.0.0.0:$HOST_PORT"  "port <cid> 80"
    run_podman port myweb 80/tcp
    is "$output" "0.0.0.0:$HOST_PORT"  "port <cid> 80/tcp"

    run_podman 125 port myweb 99/tcp
    is "$output" 'Error: failed to find published port "99/tcp"'

    # Clean up
    run_podman stop -t 1 myweb
    run_podman rm myweb
    run_podman rmi busybox
}

# vim: filetype=sh
