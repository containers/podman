#!/usr/bin/env bats   -*- bats -*-
#
# Test podman play
#

load helpers

# This is a long ugly way to clean up pods and remove the pause image
function teardown() {
    run_podman pod rm -f -a
    run_podman rm -f -a
    run_podman image list --format '{{.ID}} {{.Repository}}'
    while read id name; do
        if [[ "$name" =~ /pause ]]; then
            run_podman rmi $id
        fi
    done <<<"$output"

    basic_teardown
}

testYaml="
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: test_pod
spec:
  containers:
  - command:
    - sleep
    - "100"
    env:
    - name: PATH
      value: /usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
    - name: TERM
      value: xterm
    - name: container
      value: podman
    image: $IMAGE
    name: test
    resources: {}
    securityContext:
      runAsUser: 1000
      runAsGroup: 3000
      fsGroup: 2000
      allowPrivilegeEscalation: true
      capabilities: {}
      privileged: false
      seLinuxOptions:
         level: "s0:c1,c2"
      readOnlyRootFilesystem: false
    workingDir: /
status: {}
"

@test "podman play with stdin" {
    echo "$testYaml" > $PODMAN_TMPDIR/test.yaml
    run_podman play kube - < $PODMAN_TMPDIR/test.yaml
    run_podman pod rm -f test_pod
}

@test "podman play" {
    echo "$testYaml" > $PODMAN_TMPDIR/test.yaml
    run_podman play kube $PODMAN_TMPDIR/test.yaml
    run_podman pod rm -f test_pod
}
