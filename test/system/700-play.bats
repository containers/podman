#!/usr/bin/env bats   -*- bats -*-
#
# Test podman play
#

load helpers

# This is a long ugly way to clean up pods and remove the pause image
function teardown() {
    run_podman pod rm -t 0 -f -a
    run_podman rm -t 0 -f -a
    run_podman image list --format '{{.ID}} {{.Repository}}'
    while read id name; do
        if [[ "$name" =~ /podman-pause ]]; then
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
    - \"100\"
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
         level: \"s0:c1,c2\"
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
       is "$output" "${RELABEL} $TESTDIR" "selinux relabel should have happened"
    fi

    # Make sure that the K8s pause image isn't pulled but the local podman-pause is built.
    run_podman images
    run_podman 1 image exists k8s.gcr.io/pause
    run_podman version --format "{{.Server.Version}}-{{.Server.Built}}"
    run_podman image exists localhost/podman-pause:$output

    run_podman stop -a -t 0
    run_podman pod rm -t 0 -f test_pod
}

@test "podman play" {
    TESTDIR=$PODMAN_TMPDIR/testdir
    mkdir -p $TESTDIR
    echo "$testYaml" | sed "s|TESTDIR|${TESTDIR}|g" > $PODMAN_TMPDIR/test.yaml
    run_podman play kube $PODMAN_TMPDIR/test.yaml
    if [ -e /usr/sbin/selinuxenabled -a /usr/sbin/selinuxenabled ]; then
       run ls -Zd $TESTDIR
       is "$output" "${RELABEL} $TESTDIR" "selinux relabel should have happened"
    fi

    run_podman stop -a -t 0
    run_podman pod rm -t 0 -f test_pod
}

@test "podman play --network" {
    TESTDIR=$PODMAN_TMPDIR/testdir
    mkdir -p $TESTDIR
    echo "$testYaml" | sed "s|TESTDIR|${TESTDIR}|g" > $PODMAN_TMPDIR/test.yaml
    run_podman 125 play kube --network host $PODMAN_TMPDIR/test.yaml
    is "$output" ".*invalid value passed to --network: bridge or host networking must be configured in YAML" "podman plan-network should fail with --network host"
    run_podman play kube --network slirp4netns:port_handler=slirp4netns $PODMAN_TMPDIR/test.yaml
    run_podman pod inspect --format {{.InfraContainerID}} "${lines[1]}"
    infraID="$output"
    run_podman container inspect --format "{{.HostConfig.NetworkMode}}" $infraID
    is "$output" "slirp4netns" "network mode slirp4netns is set for the container"

    run_podman stop -a -t 0
    run_podman pod rm -t 0 -f test_pod

    run_podman play kube --network none $PODMAN_TMPDIR/test.yaml
    run_podman pod inspect --format {{.InfraContainerID}} "${lines[1]}"
    infraID="$output"
    run_podman container inspect --format "{{.HostConfig.NetworkMode}}" $infraID
    is "$output" "none" "network mode none is set for the container"

    run_podman stop -a -t 0
    run_podman pod rm -t 0 -f test_pod
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

    run_podman stop -a -t 0
    run_podman pod rm -t 0 -f test_pod
    run_podman rmi -f userimage:latest
}

@test "podman play --build --context-dir" {
   skip_if_remote "--build is not supported in context remote"
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
    image: quay.io/libpod/userimage
    name: test
    resources: {}
status: {}
"

mkdir -p $PODMAN_TMPDIR/userimage
cat > $PODMAN_TMPDIR/userimage/Containerfile << _EOF
from $IMAGE
USER bin
_EOF

    echo "$testUserYaml" > $PODMAN_TMPDIR/test.yaml
    run_podman 125 play kube --build --start=false $PODMAN_TMPDIR/test.yaml
    run_podman play kube --replace --context-dir=$PODMAN_TMPDIR --build --start=false $PODMAN_TMPDIR/test.yaml
    run_podman inspect --format "{{ .Config.User }}" test_pod-test
    is "$output" bin "expect container within pod to run as the bin user"

    run_podman stop -a -t 0
    run_podman pod rm -t 0 -f test_pod
    run_podman rmi -f userimage:latest

    cd $PODMAN_TMPDIR
    run_podman play kube --replace --build --start=false $PODMAN_TMPDIR/test.yaml
    run_podman inspect --format "{{ .Config.User }}" test_pod-test
    is "$output" bin "expect container within pod to run as the bin user"

    run_podman stop -a -t 0
    run_podman pod rm -t 0 -f test_pod
    run_podman rmi -f userimage:latest
}

@test "podman play --annotation" {
    TESTDIR=$PODMAN_TMPDIR/testdir
    RANDOMSTRING=$(random_string 15)
    mkdir -p $TESTDIR
    echo "$testYaml" | sed "s|TESTDIR|${TESTDIR}|g" > $PODMAN_TMPDIR/test.yaml
    run_podman play kube --annotation "name=$RANDOMSTRING" $PODMAN_TMPDIR/test.yaml
    run_podman inspect --format "{{ .Config.Annotations }}" test_pod-test
    is "$output" ".*name:$RANDOMSTRING" "Annotation should be added to pod"

    run_podman stop -a -t 0
    run_podman pod rm -t 0 -f test_pod
}

@test "podman play --annotation > Max" {
    TESTDIR=$PODMAN_TMPDIR/testdir
    RANDOMSTRING=$(random_string 65)
    mkdir -p $TESTDIR
    echo "$testYaml" | sed "s|TESTDIR|${TESTDIR}|g" > $PODMAN_TMPDIR/test.yaml
    run_podman 125 play kube --annotation "name=$RANDOMSTRING" $PODMAN_TMPDIR/test.yaml
    assert "$output" =~ "annotation exceeds maximum size, 63, of kubernetes annotation:" "Expected to fail with Length greater than 63"
}

@test "podman play Yaml with annotation > Max" {
   RANDOMSTRING=$(random_string 65)
   testBadYaml="
apiVersion: v1
kind: Pod
metadata:
  annotations:
    test: ${RANDOMSTRING}
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
    image: quay.io/libpod/userimage
    name: test
    resources: {}
status: {}
"
    TESTDIR=$PODMAN_TMPDIR/testdir
    mkdir -p $TESTDIR
    echo "$testBadYaml" | sed "s|TESTDIR|${TESTDIR}|g" > $PODMAN_TMPDIR/test.yaml

    run_podman 125 play kube - < $PODMAN_TMPDIR/test.yaml
    assert "$output" =~ "invalid annotation \"test\"=\"$RANDOMSTRING\"" "Expected to fail with annotation length greater than 63"
}

@test "podman play kube - default log driver" {
    TESTDIR=$PODMAN_TMPDIR/testdir
    mkdir -p $TESTDIR
    echo "$testYaml" | sed "s|TESTDIR|${TESTDIR}|g" > $PODMAN_TMPDIR/test.yaml
    # Get the default log driver
    run_podman info --format "{{.Host.LogDriver}}"
    default_driver=$output

    # Make sure that the default log driver is used
    run_podman play kube $PODMAN_TMPDIR/test.yaml
    run_podman inspect --format "{{.HostConfig.LogConfig.Type}}" test_pod-test
    is "$output" "$default_driver" "play kube uses default log driver"

    run_podman stop -a -t 0
    run_podman pod rm -t 0 -f test_pod
}
