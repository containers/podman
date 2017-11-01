# Fedora and RHEL Integration and End-to-End Tests

This directory contains playbooks to set up for and run the integration and
end-to-end tests for CRI-O on RHEL and Fedora hosts. Two entrypoints exist:

 - `main.yml`: sets up the machine and runs tests
 - `results.yml`: gathers test output to `/tmp/artifacts`

When running `main.yml`, three tags are present:

 - `setup`: run all tasks to set up the system for testing
 - `e2e`: build CRI-O from source and run Kubernetes node E2Es
 - `integration`: build CRI-O from source and run the local integration suite

The playbooks assume the following things about your system:

 - on RHEL, the server and extras repos are configured and certs are present
 - `ansible` is installed and the host is boot-strapped to allow `ansible` to run against it
 - the `$GOPATH` is set and present for all shells (*e.g.* written in `/etc/environment`)
 - CRI-O is checked out to the correct state at `${GOPATH}/src/github.com/kubernetes-incubator/cri-o`
 - the user running the playbook has access to passwordless `sudo`