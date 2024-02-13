#!/usr/bin/env bats   -*- bats -*-
#
# Test podman play
#

load helpers
load helpers.network
load helpers.registry

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
    if selinux_enabled; then
       run ls -Zd $TESTDIR
       is "$output" "${RELABEL} $TESTDIR" "selinux relabel should have happened"
    fi

    # Make sure that the K8s pause image isn't pulled but the local podman-pause is built.
    run_podman images
    run_podman 1 image exists k8s.gcr.io/pause
    run_podman 1 image exists registry.k8s.io/pause
    run_podman image exists $(pause_image)

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
    if selinux_enabled; then
       run ls -Zd $TESTDIR
       is "$output" "${RELABEL} $TESTDIR" "selinux relabel should have happened"
    fi

    # Now rerun twice to make sure nothing gets removed
    run_podman 125 play kube $PODMAN_TMPDIR/test.yaml
    is "$output" ".* is in use: pod already exists"
    run_podman 125 play kube $PODMAN_TMPDIR/test.yaml
    is "$output" ".* is in use: pod already exists"

    run_podman stop -a -t 0
    run_podman pod rm -t 0 -f test_pod
}

# helper function: writes a yaml file with customizable values
function _write_test_yaml() {
    local outfile=$PODMAN_TMPDIR/test.yaml

    # Function args must all be of the form 'keyword=value' (value may be null)
    local annotations=
    local labels="app: test"
    local name="test_pod"
    local command=""
    local image="$IMAGE"
    local ctrname="test"
    for i;do
        # This will error on 'foo=' (no value). That's totally OK.
        local value=$(expr "$i" : '[^=]*=\(.*\)')
        case "$i" in
            annotations=*)   annotations="$value" ;;
            labels=*)        labels="$value"      ;;
            name=*)          name="$value"        ;;
            command=*)       command="$value"     ;;
            image=*)         image="$value"       ;;
            ctrname=*)       ctrname="$value"     ;;
            *)               die "_write_test_yaml: cannot grok '$i'" ;;
        esac
    done

    # These three header lines are common to all yamls.
    # Note: use >> (append), not > (overwrite), for multi-pod test
    cat >>$outfile <<EOF
apiVersion: v1
kind: Pod
metadata:
EOF

    if [[ -n "$annotations" ]]; then
        echo "  annotations:"   >>$outfile
        echo "    $annotations" >>$outfile
    fi
    if [[ -n "$labels" ]]; then
        echo "  labels:"        >>$outfile
        echo "    $labels"      >>$outfile
    fi
    if [[ -n "$name" ]]; then
        echo "  name: $name"    >>$outfile
    fi

    # We always have spec and container lines...
    echo "spec:"                >>$outfile
    echo "  containers:"        >>$outfile
    # ...but command is optional. If absent, assume our caller will fill it in.
    if [[ -n "$command" ]]; then
        cat <<EOF               >>$outfile
  - command:
    - $command
    image: $image
    name: $ctrname
    resources: {}
status: {}
EOF
    fi
}

@test "podman play --service-container" {
    skip_if_remote "service containers only work locally"

    # Create the YAMl file
    yaml_source="$PODMAN_TMPDIR/test.yaml"
    _write_test_yaml command=top

    # Run `play kube` in the background as it will wait for the service
    # container to exit.
    timeout --foreground -v --kill=10 60 \
        $PODMAN play kube --service-container=true --log-driver journald $yaml_source &>/dev/null &

    # Wait for the container to be running
    container_a=test_pod-test
    container_running=
    for i in $(seq 1 20); do
        run_podman "?" container wait $container_a --condition="running"
        if [[ $status == 0 ]]; then
            container_running=1
            break
        fi
        sleep 0.5
        # Just for debugging
        run_podman ps -a
    done
    if [[ -z "$container_running" ]]; then
        die "container $container_a did not start"
    fi

    # The name of the service container is predictable: the first 12 characters
    # of the hash of the YAML file followed by the "-service" suffix
    yaml_sha=$(sha256sum $yaml_source)
    service_container="${yaml_sha:0:12}-service"

    # Make sure that the service container exists and runs.
    run_podman container inspect $service_container --format "{{.State.Running}}"
    is "$output" "true"

    run_podman container inspect $service_container --format '{{.Config.StopTimeout}}'
    is "$output" "10" "StopTimeout should be initialized to 10"

    # Stop the *main* container and make sure that
    #  1) The pod transitions to Exited
    #  2) The service container is stopped
    #  #) The service container is marked as a service container
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
    run_podman 125 container rm --force $service_container
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
    is "$output" ".*Error: no such object: \"test_pod-test\""
    run_podman pod rm -a
    run_podman rm -a
}

@test "podman kube play read-only" {
    YAML=$PODMAN_TMPDIR/test.yml

    # --restart=no is crucial: without that, the "podman wait" below
    # will spin for indeterminate time.
    run_podman create --pod new:pod1         --restart=no --name test1 $IMAGE touch /testrw
    run_podman create --pod pod1 --read-only --restart=no --name test2 $IMAGE touch /testro
    run_podman create --pod pod1 --read-only --restart=no --name test3 $IMAGE sh -c "echo "#!echo hi" > /tmp/testtmp; chmod +x /tmp/test/tmp; /tmp/testtmp"

    # Generate and run from yaml. (The "cat" is for debugging failures)
    run_podman kube generate pod1 -f $YAML
    cat $YAML
    run_podman kube play --replace $YAML

    # Wait for all containers and check their exit statuses
    run_podman wait pod1-test1 pod1-test2 pod1-test3
    is "${lines[0]}" 0 "exit status: touch /file on read/write container"
    is "${lines[1]}" 1 "exit status: touch /file on read-only container"
    is "${lines[2]}" 0 "exit status: touch on /tmp is always ok, even on read-only container"

    # Confirm config settings
    run_podman container inspect --format '{{.HostConfig.ReadonlyRootfs}}' pod1-test1 pod1-test2 pod1-test3
    is "${lines[0]}" "false"  "ReadonlyRootfs - container 1"
    is "${lines[1]}" "true"   "ReadonlyRootfs - container 2"
    is "${lines[2]}" "true"   "ReadonlyRootfs - container 3"

    # Clean up
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

    # --restart=no is crucial: without that, the "podman wait" below
    # will spin for indeterminate time.
    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman create --pod new:pod1 --read-only=false --restart=no --name test1 $IMAGE touch /testrw
    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman create --pod pod1                       --restart=no --name test2 $IMAGE touch /testro
    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman create --pod pod1                       --restart=no --name test3 $IMAGE touch /tmp/testtmp

    # Inspect settings in created containers
    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman container inspect --format '{{.HostConfig.ReadonlyRootfs}}' test1 test2 test3
    is "${lines[0]}" "false"  "ReadonlyRootfs - container 1, created"
    is "${lines[1]}" "true"   "ReadonlyRootfs - container 2, created"
    is "${lines[2]}" "true"   "ReadonlyRootfs - container 3, created"

    # Now generate and run kube.yaml on a machine without the defaults set
    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman kube generate pod1 -f $YAML
    cat $YAML

    run_podman kube play --replace $YAML

    # Wait for all containers and check their exit statuses
    run_podman wait pod1-test1 pod1-test2 pod1-test3
    is "${lines[0]}" 0 "exit status: touch /file on read/write container"
    is "${lines[1]}" 1 "exit status: touch /file on read-only container"
    is "${lines[2]}" 0 "exit status: touch on /tmp is always ok, even on read-only container"

    # Confirm settings again
    run_podman container inspect --format '{{.HostConfig.ReadonlyRootfs}}' pod1-test1 pod1-test2 pod1-test3
    is "${lines[0]}" "false"  "ReadonlyRootfs - container 1, post-run"
    is "${lines[1]}" "true"   "ReadonlyRootfs - container 2, post-run"
    is "${lines[2]}" "true"   "ReadonlyRootfs - container 3, post-run"

    # Clean up
    run_podman kube down - < $YAML
    run_podman 1 container exists pod1-test1
    run_podman 1 container exists pod1-test2
    run_podman 1 container exists pod1-test3
}

@test "podman play with user from image" {
    TESTDIR=$PODMAN_TMPDIR/testdir
    mkdir -p $TESTDIR

    _write_test_yaml command=id image=userimage

cat > $PODMAN_TMPDIR/Containerfile << _EOF
from $IMAGE
USER bin
_EOF

    # Unset the PATH during build and make sure that all default env variables
    # are correctly set for the created container.
    run_podman build --unsetenv PATH -t userimage $PODMAN_TMPDIR
    run_podman image inspect userimage --format "{{.Config.Env}}"
    is "$output" "\[\]" "image does not set PATH - env is empty"

    run_podman play kube --start=false $PODMAN_TMPDIR/test.yaml
    run_podman inspect --format "{{ .Config.User }}" test_pod-test
    is "$output" bin "expect container within pod to run as the bin user"
    run_podman inspect --format "{{ .Config.Env }}" test_pod-test
    is "$output" ".*PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin.*" "expect PATH to be set"
    is "$output" ".*container=podman.*" "expect container to be set"

    run_podman stop -a -t 0
    run_podman pod rm -t 0 -f test_pod
    run_podman rmi -f userimage:latest
}

@test "podman play --build --context-dir" {
    skip_if_remote "--build is not supported in context remote"

    mkdir -p $PODMAN_TMPDIR/userimage
    cat > $PODMAN_TMPDIR/userimage/Containerfile << _EOF
from $IMAGE
USER bin
_EOF

    _write_test_yaml command=id image=quay.io/libpod/userimage
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

# Occasionally a remnant storage container is left behind which causes
# podman play kube --replace to fail. This tests created a conflicting
# storage container name using buildah to make sure --replace, still
# functions proplery by removing the storage container.
@test "podman kube play --replace external storage" {
    TESTDIR=$PODMAN_TMPDIR/testdir
    mkdir -p $TESTDIR
    echo "$testYaml" | sed "s|TESTDIR|${TESTDIR}|g" > $PODMAN_TMPDIR/test.yaml
    run_podman play kube $PODMAN_TMPDIR/test.yaml
    # Force removal of container
    run_podman rm --force -t0 test_pod-test
    # Create external container using buildah with same name
    buildah from --name test_pod-test $IMAGE
    # --replace deletes the buildah container and replace it with new one
    run_podman play kube --replace $PODMAN_TMPDIR/test.yaml

    run_podman stop -a -t 0
    run_podman pod rm -t 0 -f test_pod
    run_podman rmi -f userimage:latest
}

@test "podman kube --annotation" {
    TESTDIR=$PODMAN_TMPDIR/testdir
    RANDOMSTRING=$(random_string 15)
    ANNOTATION_WITH_COMMA="comma,$(random_string 5)"
    mkdir -p $TESTDIR
    echo "$testYaml" | sed "s|TESTDIR|${TESTDIR}|g" > $PODMAN_TMPDIR/test.yaml
    run_podman kube play --annotation "name=$RANDOMSTRING"  \
        --annotation "anno=$ANNOTATION_WITH_COMMA" $PODMAN_TMPDIR/test.yaml
    run_podman inspect --format "{{ .Config.Annotations }}" test_pod-test
    is "$output" ".*name:$RANDOMSTRING" "Annotation should be added to pod"
    is "$output" ".*anno:$ANNOTATION_WITH_COMMA" "Annotation with comma should be added to pod"

    # invalid annotation
    run_podman 125 kube play --annotation "val" $PODMAN_TMPDIR/test.yaml
    assert "$output" == "Error: annotation \"val\" must include an '=' sign" "invalid annotation error"

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

@test "podman play --no-trunc --annotation > Max" {
    TESTDIR=$PODMAN_TMPDIR/testdir
    RANDOMSTRING=$(random_string 65)
    mkdir -p $TESTDIR
    echo "$testYaml" | sed "s|TESTDIR|${TESTDIR}|g" > $PODMAN_TMPDIR/test.yaml
    run_podman play kube --no-trunc --annotation "name=$RANDOMSTRING" $PODMAN_TMPDIR/test.yaml
}

@test "podman play Yaml with annotation > Max" {
   RANDOMSTRING=$(random_string 65)

   _write_test_yaml "annotations=test: ${RANDOMSTRING}" command=id
   run_podman 125 play kube - < $PODMAN_TMPDIR/test.yaml
   assert "$output" =~ "annotation \"test\"=\"$RANDOMSTRING\" value length exceeds Kubernetes max 63" "Expected to fail with annotation length greater than 63"
}

@test "podman play Yaml --no-trunc with annotation > Max" {
   RANDOMSTRING=$(random_string 65)

   _write_test_yaml "annotations=test: ${RANDOMSTRING}" command=id
   run_podman play kube --no-trunc - < $PODMAN_TMPDIR/test.yaml
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
    is "$output" ".*Error: no such object: \"test_pod-test\""
    run_podman pod rm -a
    run_podman rm -a
}

@test "podman kube play - URL" {
    TESTDIR=$PODMAN_TMPDIR/testdir
    mkdir -p $TESTDIR
    echo "$testYaml" | sed "s|TESTDIR|${TESTDIR}|g" > $PODMAN_TMPDIR/test.yaml
    echo READY                                      > $PODMAN_TMPDIR/ready

    HOST_PORT=$(random_free_port)
    SERVER=http://127.0.0.1:$HOST_PORT

    run_podman run -d --name myyaml -p "$HOST_PORT:80" \
               -v $PODMAN_TMPDIR/test.yaml:/var/www/testpod.yaml:Z \
               -v $PODMAN_TMPDIR/ready:/var/www/ready:Z \
               -w /var/www \
               $IMAGE /bin/busybox-extras httpd -f -p 80

    wait_for_port 127.0.0.1 $HOST_PORT
    wait_for_command_output "curl -s -S $SERVER/ready" "READY"

    run_podman kube play $SERVER/testpod.yaml
    run_podman inspect test_pod-test --format "{{.State.Running}}"
    is "$output" "true"
    run_podman kube down $SERVER/testpod.yaml
    run_podman 125 inspect test_pod-test
    is "$output" ".*Error: no such object: \"test_pod-test\""

    run_podman pod rm -a -f
    run_podman rm -a -f -t0
}

@test "podman play with init container" {
    _write_test_yaml command=
    cat >>$PODMAN_TMPDIR/test.yaml <<EOF
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
EOF

    run_podman kube play $PODMAN_TMPDIR/test.yaml
    assert "$output" !~ "level=" "init containers should not generate logrus.Error"
    run_podman inspect --format "{{.State.ExitCode}}" test_pod-testCtr
    is "$output" "0" "init container should have created /dev/shm/test1"

    run_podman kube down $PODMAN_TMPDIR/test.yaml
}

@test "podman kube play - hostport" {
    HOST_PORT=$(random_free_port)
    _write_test_yaml
    cat >>$PODMAN_TMPDIR/test.yaml <<EOF
    - name: server
      image: $IMAGE
      ports:
        - name: hostp
          hostPort: $HOST_PORT
EOF

    run_podman kube play $PODMAN_TMPDIR/test.yaml
    run_podman pod inspect test_pod --format "{{.InfraConfig.PortBindings}}"
    assert "$output" = "map[$HOST_PORT/tcp:[{ $HOST_PORT}]]"
    run_podman kube down $PODMAN_TMPDIR/test.yaml

    run_podman pod rm -a -f
    run_podman rm -a -f
}

@test "podman kube play - multi-pod YAML" {
    skip_if_remote "service containers only work locally"
    skip_if_journald_unavailable

    # Create the YAMl file, with two pods, each with one container
    yaml_source="$PODMAN_TMPDIR/test.yaml"
    for n in 1 2;do
        _write_test_yaml labels="app: pod$n" name="pod$n" ctrname="ctr$n" command=top

        # Separator between two yaml halves
        if [[ $n = 1 ]]; then
            echo "---" >>$yaml_source
        fi
    done

    # Run `play kube` in the background as it will wait for the service
    # container to exit.
    timeout --foreground -v --kill=10 60 \
        $PODMAN play kube --service-container=true --log-driver journald $yaml_source &>/dev/null &

    # The name of the service container is predictable: the first 12 characters
    # of the hash of the YAML file followed by the "-service" suffix
    yaml_sha=$(sha256sum $yaml_source)
    service_container="${yaml_sha:0:12}-service"
    # Wait for the containers to be running
    container_1=pod1-ctr1
    container_2=pod2-ctr2
    containers_running=
    for i in $(seq 1 20); do
        run_podman "?" container wait $container_1 $container_2 $service_container --condition="running"
        if [[ $status == 0 ]]; then
            containers_running=1
            break
        fi
        sleep 0.5
        # Just for debugging
        run_podman ps -a
    done
    if [[ -z "$containers_running" ]]; then
        die "container $container_1, $container_2 and/or $service_container did not start"
    fi

    # Stop the pods, make sure that no ugly error logs show up and that the
    # service container will implicitly get stopped as well
    run_podman pod stop pod1 pod2
    assert "$output" !~ "Stopping"
    _ensure_container_running $service_container false

    run_podman kube down $yaml_source
}

@test "podman kube generate filetype" {
    YAML=$PODMAN_TMPDIR/test.yml
    run_podman create --pod new:pod1 --security-opt label=level:s0:c1,c2 --security-opt label=filetype:usr_t -v myvol:/myvol --name test1 $IMAGE true
    run_podman kube generate pod1 -f $YAML
    run cat $YAML
    is "$output" ".*filetype: usr_t" "Generated YAML file should contain filetype usr_t"
    run_podman pod rm --force pod1
    run_podman volume rm -t -1 myvol --force

    run_podman kube play $YAML
    if selinux_enabled; then
        run_podman inspect pod1-test1 --format "{{ .MountLabel }}"
        is "$output" "system_u:object_r:usr_t:s0:c1,c2" "Generated container should use filetype usr_t"
        run_podman volume inspect myvol --format '{{ .Mountpoint }}'
        path=${output}
        run ls -Zd $path
        is "$output" "system_u:object_r:usr_t:s0 $path" "volume should be labeled with usr_t type"
    fi
    run_podman kube down $YAML
    run_podman volume rm myvol --force
}

# kube play --wait=true, where we clear up the created containers, pods, and volumes when a kill or sigterm is triggered
@test "podman kube play --wait with siginterrupt" {
    cname=c$(random_string 15)
    fname="/tmp/play_kube_wait_$(random_string 6).yaml"
    run_podman container create --name $cname $IMAGE top
    run_podman kube generate -f $fname $cname

    # delete the container we generated from
    run_podman rm -f $cname

    # force a timeout to happen so that the kube play command is killed
    # and expect the timeout code 124 to happen so that we can clean up
    local t0=$SECONDS
    PODMAN_TIMEOUT=15 run_podman 124 kube play --wait $fname
    local t1=$SECONDS
    local delta_t=$((t1 - t0))
    assert $delta_t -le 20 \
           "podman kube play did not get killed within 10 seconds"

    # there should be no containers running or created
    run_podman ps -aq
    is "$output" "" "There should be no containers"
    run_podman rmi $(pause_image)
}

@test "podman kube play --wait - wait for pod to exit" {
    fname="/tmp/play_kube_wait_$(random_string 6).yaml"
    echo "
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: test_pod
spec:
  restartPolicy: Never
  containers:
    - name: server
      image: $IMAGE
      command:
      - echo
      - "hello"
" > $fname

    run_podman kube play --wait $fname

    # debug to see what container is being left behind after the cleanup
    # there should be no containers running or created
    run_podman ps -a --noheading
    is "$output" "" "There should be no containers"
    run_podman pod ps
    run_podman rmi $(pause_image)
}

@test "podman kube play with configmaps" {
    configmap_file=${PODMAN_TMPDIR}/play_kube_configmap_configmaps$(random_string 6),withcomma.yaml
    echo "
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: foo
data:
  value: foo
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: bar
data:
  value: bar
" > $configmap_file

    pod_file=${PODMAN_TMPDIR}/play_kube_configmap_pod$(random_string 6).yaml
    echo "
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: test_pod
spec:
  restartPolicy: Never
  containers:
  - name: server
    image: $IMAGE
    env:
    - name: FOO
      valueFrom:
        configMapKeyRef:
          name: foo
          key: value
    - name: BAR
      valueFrom:
        configMapKeyRef:
          name: bar
          key: value
    command:
    - /bin/sh
    args:
    - -c
    - "echo \$FOO:\$BAR"
" > $pod_file

    run_podman kube play --configmap=$configmap_file $pod_file
    run_podman wait test_pod-server
    run_podman logs test_pod-server
    is "$output" "foo:bar"

    run_podman kube down $pod_file
}

@test "podman kube with --authfile=/tmp/bogus" {
    TESTDIR=$PODMAN_TMPDIR/testdir
    mkdir -p $TESTDIR
    echo "$testYaml" | sed "s|TESTDIR|${TESTDIR}|g" > $PODMAN_TMPDIR/test.yaml
    bogus=$PODMAN_TMPDIR/bogus-authfile

    run_podman 125 kube play --authfile=$bogus - < $PODMAN_TMPDIR/test.yaml
    is "$output" "Error: credential file is not accessible: stat $bogus: no such file or directory" "$command should fail with not such file"
}

@test "podman kube play with umask from containers.conf" {
    skip_if_remote "remote does not support CONTAINERS_CONF*"
    YAML=$PODMAN_TMPDIR/test.yaml

    containersConf=$PODMAN_TMPDIR/containers.conf
    touch $containersConf
    cat >$containersConf <<EOF
[containers]
umask = "0472"
EOF

    ctr="ctr"
    ctrInPod="ctr-pod-ctr"

    run_podman create --restart never --name $ctr $IMAGE sh -c "touch /umask-test;stat -c '%a' /umask-test"
    run_podman kube generate -f $YAML $ctr
    CONTAINERS_CONF_OVERRIDE="$containersConf" run_podman kube play $YAML
    run_podman container inspect --format '{{ .Config.Umask }}' $ctrInPod
    is "${output}" "0472"
    # Confirm that umask actually takes effect
    run_podman logs $ctrInPod
    is "$output" "204" "stat() on created file"

    run_podman kube down $YAML
    run_podman pod rm -a
    run_podman rm -a
}

@test "podman kube generate tmpfs on /tmp" {
      KUBE=$PODMAN_TMPDIR/kube.yaml
      run_podman create --name test $IMAGE sleep 100
      run_podman kube generate test -f $KUBE
      run_podman kube play $KUBE
      run_podman exec test-pod-test sh -c "mount | grep /tmp"
      assert "$output" !~ "noexec" "mounts on /tmp should not be noexec"
      run_podman kube down $KUBE
      run_podman pod rm -a -f -t 0
      run_podman rm -a -f -t 0
}

@test "podman kube play - pull policy" {
    skip_if_remote "pull debug logs only work locally"

    yaml_source="$PODMAN_TMPDIR/test.yaml"
    _write_test_yaml command=true

    # Exploit a debug message to make sure the expected pull policy is used
    run_podman --debug kube play $yaml_source
    assert "$output" =~ "Pulling image $IMAGE \(policy\: missing\)" "default pull policy is missing"
    run_podman kube down $yaml_source

    local_image="localhost/name:latest"
    run_podman tag $IMAGE $local_image
    rm $yaml_source
    _write_test_yaml command=true image=$local_image

    run_podman --debug kube play $yaml_source
    assert "$output" =~ "Pulling image $local_image \(policy\: newer\)" "pull policy is set to newhen pulling latest tag"
    run_podman kube down $yaml_source

    run_podman rmi $local_image
}

@test "podman kube play healthcheck should wait initialDelaySeconds before updating status (healthy)" {
    fname="$PODMAN_TMPDIR/play_kube_healthy_$(random_string 6).yaml"
    echo "
apiVersion: v1
kind: Pod
metadata:
  labels:
  name: liveness-exec
spec:
  containers:
  - name: liveness
    image: $IMAGE
    args:
    - /bin/sh
    - -c
    - touch /tmp/healthy && sleep 100
    livenessProbe:
      exec:
        command:
        - cat
        - /tmp/healthy
      initialDelaySeconds: 3
      failureThreshold: 1
      periodSeconds: 1
" > $fname

    run_podman kube play $fname
    ctrName="liveness-exec-liveness"

    # Keep checking status. For the first 2 seconds it must be 'starting'
    t0=$SECONDS
    while [[ $SECONDS -le $((t0 + 2)) ]]; do
        run_podman inspect $ctrName --format "1-{{.State.Health.Status}}"
        assert "$output" == "1-starting" "Health.Status at $((SECONDS - t0))"
        sleep 0.5
    done

    # After 3 seconds it may take another second to go healthy. Wait.
    t0=$SECONDS
    while [[ $SECONDS -le $((t0 + 3)) ]]; do
        run_podman inspect $ctrName --format "2-{{.State.Health.Status}}"
        if [[ "$output" = "2-healthy" ]]; then
            break;
        fi
        sleep 0.5
    done
    assert $output == "2-healthy" "After 3 seconds"

    run_podman kube down $fname
    run_podman pod rm -a
    run_podman rm -a
}

@test "podman kube play healthcheck should wait initialDelaySeconds before updating status (unhealthy)" {
    fname="$PODMAN_TMPDIR/play_kube_unhealthy_$(random_string 6).yaml"
    echo "
apiVersion: v1
kind: Pod
metadata:
  labels:
  name: liveness-exec
spec:
  containers:
  - name: liveness
    image: $IMAGE
    args:
    - /bin/sh
    - -c
    - touch /tmp/healthy && sleep 100
    livenessProbe:
      exec:
        command:
        - cat
        - /tmp/randomfile
      initialDelaySeconds: 3
      failureThreshold: 1
      periodSeconds: 1
" > $fname

    run_podman kube play $fname
    ctrName="liveness-exec-liveness"

    # Keep checking status. For the first 2 seconds it must be 'starting'
    t0=$SECONDS
    while [[ $SECONDS -le $((t0 + 2)) ]]; do
        run_podman inspect $ctrName --format "1-{{.State.Health.Status}}"
        assert "$output" == "1-starting" "Health.Status at $((SECONDS - t0))"
        sleep 0.5
    done

    # After 3 seconds it may take another second to go unhealthy. Wait.
    t0=$SECONDS
    while [[ $SECONDS -le $((t0 + 3)) ]]; do
        run_podman inspect $ctrName --format "2-{{.State.Health.Status}}"
        if [[ "$output" = "2-unhealthy" ]]; then
            break;
        fi
        sleep 0.5
    done
    assert $output == "2-unhealthy" "After 3 seconds"

    run_podman kube down $fname
    run_podman pod rm -a
    run_podman rm -a
}

@test "podman play --build private registry" {
    skip_if_remote "--build is not supported in context remote"

    local registry=localhost:${PODMAN_LOGIN_REGISTRY_PORT}
    local from_image=$registry/quadlet_image_test:$(random_string)
    local authfile=$PODMAN_TMPDIR/authfile.json

    mkdir -p $PODMAN_TMPDIR/userimage
    cat > $PODMAN_TMPDIR/userimage/Containerfile << _EOF
from $from_image
USER bin
_EOF

    # Start the registry and populate the authfile that we can use for the test.
    start_registry
    run_podman login --authfile=$authfile \
        --tls-verify=false \
        --username ${PODMAN_LOGIN_USER} \
        --password ${PODMAN_LOGIN_PASS} \
        $registry

    # Push the test image to the registry
    run_podman image tag $IMAGE $from_image
    run_podman image push --tls-verify=false --authfile=$authfile $from_image

    # Remove the local image to make sure it will be pulled again
    run_podman image rm --ignore $from_image

    _write_test_yaml command=id image=userimage
    run_podman 125 play kube --build --start=false $PODMAN_TMPDIR/test.yaml
    assert "$output" "=~" \
        "Error: short-name resolution enforced but cannot prompt without a TTY|Resolving \"userimage\" using unqualified-search registries" \
        "The error message does match any of the expected ones"

    run_podman play kube --replace --context-dir=$PODMAN_TMPDIR --tls-verify=false --authfile=$authfile --build --start=false $PODMAN_TMPDIR/test.yaml
    run_podman inspect --format "{{ .Config.User }}" test_pod-test
    is "$output" bin "expect container within pod to run as the bin user"

    run_podman stop -a -t 0
    run_podman pod rm -t 0 -f test_pod
    run_podman rmi -f userimage:latest $from_image
}
