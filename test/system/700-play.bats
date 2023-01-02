#!/usr/bin/env bats   -*- bats -*-
#
# Test podman play
#

load helpers
load helpers.network

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

@test "podman kube with stdin" {
    TESTDIR=$PODMAN_TMPDIR/testdir
    mkdir -p $TESTDIR
    echo "$testYaml" | sed "s|TESTDIR|${TESTDIR}|g" > $PODMAN_TMPDIR/test.yaml

    run_podman kube play - < $PODMAN_TMPDIR/test.yaml
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
    # Testing that the "podman play" cmd still works now that
    # "podman kube" is an option.
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

@test "podman play --service-container" {
    skip_if_remote "service containers only work locally"

    # Create the YAMl file
    yaml_source="$PODMAN_TMPDIR/test.yaml"
    cat >$yaml_source <<EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: test_pod
spec:
  containers:
  - command:
    - top
    image: $IMAGE
    name: test
    resources: {}
EOF
    # Run `play kube` in the background as it will wait for the service
    # container to exit.
    timeout --foreground -v --kill=10 60 \
        $PODMAN play kube --service-container=true $yaml_source &>/dev/null &

    # Wait for the container to be running
    container_a=test_pod-test
    for i in $(seq 1 20); do
        run_podman "?" container wait $container_a --condition="running"
        if [[ $status == 0 ]]; then
            break
        fi
        sleep 0.5
        # Just for debugging
        run_podman ps -a
    done
    if [[ $status != 0 ]]; then
        die "container $container_a did not start"
    fi

    # The name of the service container is predictable: the first 12 characters
    # of the hash of the YAML file followed by the "-service" suffix
    yaml_sha=$(sha256sum $yaml_source)
    service_container="${yaml_sha:0:12}-service"

    # Make sure that the service container exists and runs.
    run_podman container inspect $service_container --format "{{.State.Running}}"
    is "$output" "true"

    # Stop the *main* container and make sure that
    #  1) The pod transitions to Exited
    #  2) The service container is stopped
    #  #) The service container is marked as an service container
    run_podman stop test_pod-test
    _ensure_pod_state test_pod Exited
    _ensure_container_running $service_container false
    run_podman container inspect $service_container --format "{{.IsService}}"
    is "$output" "true"

    # Restart the pod, make sure the service is running again
    run_podman pod restart test_pod
    run_podman container inspect $service_container --format "{{.State.Running}}"
    is "$output" "true"

    # Check for an error when trying to remove the service container
    run_podman 125 container rm $service_container
    is "$output" "Error: container .* is the service container of pod(s) .* and cannot be removed without removing the pod(s)"

    # Kill the pod and make sure the service is not running
    run_podman pod kill test_pod
    _ensure_container_running $service_container false

    # Remove the pod and make sure the service is removed along with it
    run_podman pod rm test_pod
    run_podman 1 container exists $service_container
}

@test "podman kube --network" {
    TESTDIR=$PODMAN_TMPDIR/testdir
    mkdir -p $TESTDIR
    echo "$testYaml" | sed "s|TESTDIR|${TESTDIR}|g" > $PODMAN_TMPDIR/test.yaml
    run_podman kube play --network host $PODMAN_TMPDIR/test.yaml
    is "$output" "Pod:.*" "podman kube play should work with --network host"

    run_podman pod inspect --format "{{.InfraConfig.HostNetwork}}" test_pod
    is "$output" "true" ".InfraConfig.HostNetwork"
    run_podman stop -a -t 0
    run_podman pod rm -t 0 -f test_pod

    run_podman kube play --network slirp4netns:port_handler=slirp4netns $PODMAN_TMPDIR/test.yaml
    run_podman pod inspect --format {{.InfraContainerID}} "${lines[1]}"
    infraID="$output"
    run_podman container inspect --format "{{.HostConfig.NetworkMode}}" $infraID
    is "$output" "slirp4netns" "network mode slirp4netns is set for the container"

    run_podman stop -a -t 0
    run_podman pod rm -t 0 -f test_pod

    run_podman kube play --network none $PODMAN_TMPDIR/test.yaml
    run_podman pod inspect --format {{.InfraContainerID}} "${lines[1]}"
    infraID="$output"
    run_podman container inspect --format "{{.HostConfig.NetworkMode}}" $infraID
    is "$output" "none" "network mode none is set for the container"

    run_podman kube down $PODMAN_TMPDIR/test.yaml
    run_podman 125 inspect test_pod-test
    is "$output" ".*Error: inspecting object: no such object: \"test_pod-test\""
    run_podman pod rm -a
    run_podman rm -a
}

@test "podman kube play read-only" {
    YAML=$PODMAN_TMPDIR/test.yml
    run_podman create --pod new:pod1 --name test1 $IMAGE touch /testrw
    run_podman create --pod pod1 --read-only --name test2 $IMAGE touch /testro
    run_podman create --pod pod1 --read-only --name test3 $IMAGE touch /tmp/testtmp
    run_podman kube generate pod1 -f $YAML

    run_podman kube play --replace $YAML
    run_podman container inspect --format '{{.HostConfig.ReadonlyRootfs}}' pod1-test1 pod1-test2 pod1-test3
    is "$output" "false.*true.*true" "Rootfs should be read/only"

    run_podman inspect --format "{{.State.ExitCode}}" pod1-test1
    is "$output" "0" "Container / should be read/write"
    run_podman inspect --format "{{.State.ExitCode}}" pod1-test2
    is "$output" "1" "Container / should be read/only"
    run_podman inspect --format "{{.State.ExitCode}}" pod1-test3
    is "$output" "0" "/tmp in a read-only container should be read/write"

    run_podman kube down - < $YAML
    run_podman 1 container exists pod1-test1
    run_podman 1 container exists pod1-test2
    run_podman 1 container exists pod1-test3
}

@test "podman kube play read-only from containers.conf" {
    containersconf=$PODMAN_TMPDIR/containers.conf
    cat >$containersconf <<EOF
[containers]
read_only=true
EOF

    YAML=$PODMAN_TMPDIR/test.yml
    CONTAINERS_CONF="$containersconf" run_podman create --pod new:pod1 --read-only=false --name test1 $IMAGE touch /testrw
    CONTAINERS_CONF="$containersconf" run_podman create --pod pod1 --name test2 $IMAGE touch /testro
    CONTAINERS_CONF="$containersconf" run_podman create --pod pod1 --name test3 $IMAGE touch /tmp/testtmp
    CONTAINERS_CONF="$containersconf" run_podman container inspect --format '{{.HostConfig.ReadonlyRootfs}}' test1 test2 test3
    is "$output" "false.*true.*true" "Rootfs should be read/only"

    # Now generate and run kube.yaml on a machine without the defaults set
    CONTAINERS_CONF="$containersconf" run_podman kube generate pod1 -f $YAML
    cat $YAML

    run_podman kube play --replace $YAML
    run_podman container inspect --format '{{.HostConfig.ReadonlyRootfs}}' pod1-test1 pod1-test2 pod1-test3
    is "$output" "false.*true.*true" "Rootfs should be read/only"

    run_podman inspect --format "{{.State.ExitCode}}" pod1-test1
    is "$output" "0" "Container / should be read/write"
    run_podman inspect --format "{{.State.ExitCode}}" pod1-test2
    is "$output" "1" "Container / should be read/only"
    run_podman inspect --format "{{.State.ExitCode}}" pod1-test3
    is "$output" "0" "/tmp in a read-only container should be read/write"

    run_podman kube down - < $YAML
    run_podman 1 container exists pod1-test1
    run_podman 1 container exists pod1-test2
    run_podman 1 container exists pod1-test3
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

@test "podman kube --annotation" {
    TESTDIR=$PODMAN_TMPDIR/testdir
    RANDOMSTRING=$(random_string 15)
    mkdir -p $TESTDIR
    echo "$testYaml" | sed "s|TESTDIR|${TESTDIR}|g" > $PODMAN_TMPDIR/test.yaml
    run_podman kube play --annotation "name=$RANDOMSTRING" $PODMAN_TMPDIR/test.yaml
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

@test "podman kube play - default log driver" {
    TESTDIR=$PODMAN_TMPDIR/testdir
    mkdir -p $TESTDIR
    echo "$testYaml" | sed "s|TESTDIR|${TESTDIR}|g" > $PODMAN_TMPDIR/test.yaml
    # Get the default log driver
    run_podman info --format "{{.Host.LogDriver}}"
    default_driver=$output

    # Make sure that the default log driver is used
    run_podman kube play $PODMAN_TMPDIR/test.yaml
    run_podman inspect --format "{{.HostConfig.LogConfig.Type}}" test_pod-test
    is "$output" "$default_driver" "play kube uses default log driver"

    run_podman kube down $PODMAN_TMPDIR/test.yaml
    run_podman 125 inspect test_pod-test
    is "$output" ".*Error: inspecting object: no such object: \"test_pod-test\""
    run_podman pod rm -a
    run_podman rm -a
}

@test "podman kube play - URL" {
    TESTDIR=$PODMAN_TMPDIR/testdir
    mkdir -p $TESTDIR
    echo "$testYaml" | sed "s|TESTDIR|${TESTDIR}|g" > $PODMAN_TMPDIR/test.yaml

    HOST_PORT=$(random_free_port)
    SERVER=http://127.0.0.1:$HOST_PORT

    run_podman run -d --name myyaml -p "$HOST_PORT:80" \
    -v $PODMAN_TMPDIR/test.yaml:/var/www/testpod.yaml:Z \
    -w /var/www \
    $IMAGE /bin/busybox-extras httpd -f -p 80

    run_podman kube play $SERVER/testpod.yaml
    run_podman inspect test_pod-test --format "{{.State.Running}}"
    is "$output" "true"
    run_podman kube down $SERVER/testpod.yaml
    run_podman 125 inspect test_pod-test
    is "$output" ".*Error: inspecting object: no such object: \"test_pod-test\""

    run_podman pod rm -a -f
    run_podman rm -a -f
    run_podman rm -f -t0 myyaml
}

@test "podman play with init container" {
    TESTDIR=$PODMAN_TMPDIR/testdir
    mkdir -p $TESTDIR

testUserYaml="
apiVersion: v1
kind: Pod
metadata:
  name: pod
spec:
  containers:
  - command:
    - ls
    - /dev/shm/test1
    image: $IMAGE
    name: testCtr
  initContainers:
  - command:
    - touch
    - /dev/shm/test1
    image: $IMAGE
    name: initCtr
"
    echo "$testUserYaml" > $PODMAN_TMPDIR/test.yaml
    run_podman kube play $PODMAN_TMPDIR/test.yaml
    assert "$output" !~ "level=" "init containers should not generate logrus.Error"
    run_podman inspect --format "{{.State.ExitCode}}" pod-testCtr
    is "$output" "0" "init container should have created /dev/shm/test1"

    run_podman kube down $PODMAN_TMPDIR/test.yaml
}

@test "podman kube play - containerport" {
    HOST_PORT=$(random_free_port)
    echo "
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: test_pod
spec:
  containers:
    - name: server
      image: $IMAGE
      ports:
        - name: hostp
          containerPort: $HOST_PORT
" > $PODMAN_TMPDIR/testpod.yaml

    run_podman kube play $PODMAN_TMPDIR/testpod.yaml
    run_podman pod inspect test_pod --format "{{index .InfraConfig.PortBindings \"$HOST_PORT/tcp\" | len}}"
    assert "$output" = "1"
    run_podman kube down $PODMAN_TMPDIR/testpod.yaml

    run_podman pod rm -a -f
    run_podman rm -a -f
}

@test "podman kube play - containerport and replicas" {
    echo "
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test_pod
spec:
  replicas: 3
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
        - name: server
          image: $IMAGE
          ports:
            - name: hostp
              containerPort: 8080
" > "$PODMAN_TMPDIR/testpod.yaml"

    run_podman kube play "$PODMAN_TMPDIR/testpod.yaml"
    for i in $(seq 0 2); do
        run_podman pod inspect "test_pod-pod-$i" --format '{{ index .InfraConfig.PortBindings "8080/tcp" |len}}'
        assert "$output" = "1" "Expected port bindings from 8080 to exactly one container port"
    done
    run_podman kube down "$PODMAN_TMPDIR/testpod.yaml"

    run_podman pod rm -a -f
    run_podman rm -a -f
}

@test "podman kube play - hostport and replicas" {
    echo "
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test_pod
spec:
  replicas: 3
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
        - name: server
          image: $IMAGE
          ports:
            - name: hostp
              containerPort: 8080
              hostPort: 8080
" > "$PODMAN_TMPDIR/testpod.yaml"

    run_podman 125 kube play "$PODMAN_TMPDIR/testpod.yaml"
    is "$output" ".*deployment has a hostPort defined and multiple replicas are involved*"

    run_podman pod rm -a -f
    run_podman rm -a -f
}
