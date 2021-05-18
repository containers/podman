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
    volumeMounts:
    - mountPath: /testdir:z
      name: home-podman-testdir
    workingDir: /
  volumes:
  - hostPath:
      path: TESTDIR
      type: Directory
    name: home-podman-testdir
status: {}
"

RELABEL="system_u:object_r:container_file_t:s0"

@test "podman play with stdin" {
    TESTDIR=$PODMAN_TMPDIR/testdir
    mkdir -p $TESTDIR
    echo "$testYaml" | sed "s|TESTDIR|${TESTDIR}|g" > $PODMAN_TMPDIR/test.yaml
    run_podman play kube - < $PODMAN_TMPDIR/test.yaml
    if [ -e /usr/sbin/selinuxenabled -a /usr/sbin/selinuxenabled ]; then
       run ls -Zd $TESTDIR
       is "$output" ${RELABEL} "selinux relabel should have happened"
    fi
    run_podman pod rm -f test_pod
}

@test "podman play" {
    TESTDIR=$PODMAN_TMPDIR/testdir
    mkdir -p $TESTDIR
    echo "$testYaml" | sed "s|TESTDIR|${TESTDIR}|g" > $PODMAN_TMPDIR/test.yaml
    run_podman play kube $PODMAN_TMPDIR/test.yaml
    if [ -e /usr/sbin/selinuxenabled -a /usr/sbin/selinuxenabled ]; then
       run ls -Zd $TESTDIR
       is "$output" ${RELABEL} "selinux relabel should have happened"
    fi
    run_podman pod rm -f test_pod
}

@test "podman play with user from image" {
    TESTDIR=$PODMAN_TMPDIR/testdir
    mkdir -p $TESTDIR

testUserYaml="
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: test_pod
spec:
  containers:
  - command:
    - id
    env:
    - name: PATH
      value: /usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
    - name: TERM
      value: xterm
    - name: container
      value: podman
    image: userimage
    name: test
    resources: {}
status: {}
"

cat > $PODMAN_TMPDIR/Containerfile << _EOF
from $IMAGE
USER bin
_EOF

    echo "$testUserYaml" | sed "s|TESTDIR|${TESTDIR}|g" > $PODMAN_TMPDIR/test.yaml
    run_podman build -t userimage $PODMAN_TMPDIR
    run_podman play kube --start=false $PODMAN_TMPDIR/test.yaml
    run_podman inspect --format "{{ .Config.User }}" test_pod-test
    is "$output" bin "expect container within pod to run as the bin user"
    run_podman pod rm -f test_pod
}
