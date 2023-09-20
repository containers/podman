# -*- bash -*-

load helpers.bash

function setup_suite(){
    # only set up the podman farm before the first test
    run_podman system connection add --identity /home/$ROOTLESS_USER/.ssh/id_rsa test-node $ROOTLESS_USER@localhost
    run_podman farm create test-farm test-node
}

function teardown(){
    # clear out the farms after the last farm test
    run podman farm rm --all
}
