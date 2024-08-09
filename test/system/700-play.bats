#!/usr/bin/env bats   -*- bats -*-
#
# Test podman play
#

# All tests here can be run in parallel
# bats file_tags=ci:parallel

load helpers
load helpers.network
load helpers.registry

# Create and delete a pod. This gives us a pause image.
# FIXME: this should not be necessary. #23292
function setup_file() {
    podname="p-$(safename)"
    run_podman pod create $podname
    run_podman pod rm $podname
    # And now, we have a pause image, and each test does not
    # need to build their own.
}

# helper function: writes a yaml file with customizable values
function _write_test_yaml() {
    # All of these are available to our caller
    TESTYAML=$PODMAN_TMPDIR/test.yaml
    PODNAME="p-$(safename)"
    CTRNAME="c-$(safename)"
    PODCTRNAME="$PODNAME-$CTRNAME"

    # Function args must all be of the form 'keyword=value' (value may be null)
    local annotations=
    local labels="app: test"
    local command=""
    local image="$IMAGE"
    local volume=
    for i;do
        # This will error on 'foo=' (no value). That's totally OK.
        local value=$(expr "$i" : '[^=]*=\(.*\)')
        case "$i" in
            annotations=*)   annotations="$value" ;;
            labels=*)        labels="$value"      ;;
            name=*)          PODNAME="$value"     ;;
            command=*)       command="$value"     ;;
            image=*)         image="$value"       ;;
            ctrname=*)       CTRNAME="$value"     ;;
            volume=*)        volume="$value"      ;;
            *)               die "_write_test_yaml: cannot grok '$i'" ;;
        esac
    done

    # These three header lines are common to all yamls.
    # Note: use >> (append), not > (overwrite), for multi-pod test
    cat >>$TESTYAML <<EOF
apiVersion: v1
kind: Pod
metadata:
EOF

    if [[ -n "$annotations" ]]; then
        echo "  annotations:"   >>$TESTYAML
        echo "    $annotations" >>$TESTYAML
    fi
    if [[ -n "$labels" ]]; then
        echo "  labels:"        >>$TESTYAML
        echo "    $labels"      >>$TESTYAML
    fi
    if [[ -n "$PODNAME" ]]; then
        echo "  name: $PODNAME" >>$TESTYAML
    fi

    # We always have spec and container lines...
    echo "spec:"                >>$TESTYAML
    echo "  containers:"        >>$TESTYAML
    # ...but command is optional. If absent, assume our caller will fill it in.
    if [[ -n "$command" ]]; then
        cat <<EOF               >>$TESTYAML
  - command:
    - $command
    image: $image
    name: $CTRNAME
    resources: {}
EOF

        # only makes sense when command is given
        if [[ -n "$volume" ]]; then
            cat <<EOF >>$TESTYAML
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
      path: $volume
      type: Directory
    name: home-podman-testdir
EOF
        fi

        # Done.
        echo "status: {}" >>$TESTYAML
    fi

    # For debugging
    echo "# test yaml:"
    sed -e "s/^/    /g" <$TESTYAML
}

RELABEL="system_u:object_r:container_file_t:s0"

@test "podman kube with stdin" {
    TESTDIR=$PODMAN_TMPDIR/testdir
    mkdir -p $TESTDIR
    _write_test_yaml command=top volume=$TESTDIR

    run_podman kube play - < $TESTYAML
    if selinux_enabled; then
       run ls -Zd $TESTDIR
       is "$output" "${RELABEL} $TESTDIR" "selinux relabel should have happened"
    fi

    # Make sure that the K8s pause image isn't pulled but the local podman-pause is built.
    run_podman images
    run_podman 1 image exists k8s.gcr.io/pause
    run_podman 1 image exists registry.k8s.io/pause
    run_podman image exists $(pause_image)

    run_podman pod rm -t 0 -f $PODNAME
}

@test "podman play" {
    # Testing that the "podman play" cmd still works now that
    # "podman kube" is an option.
    TESTDIR=$PODMAN_TMPDIR/testdir
    mkdir -p $TESTDIR
    _write_test_yaml command=top volume=$TESTDIR
    run_podman play kube $TESTYAML
    if selinux_enabled; then
       run ls -Zd $TESTDIR
       is "$output" "${RELABEL} $TESTDIR" "selinux relabel should have happened"
    fi

    # Now rerun twice to make sure nothing gets removed
    run_podman 125 play kube $TESTYAML
    is "$output" ".* is in use: pod already exists"
    run_podman 125 play kube $TESTYAML
    is "$output" ".* is in use: pod already exists"

    run_podman pod rm -t 0 -f $PODNAME
}

@test "podman play --service-container" {
    skip_if_remote "service containers only work locally"

    # Create the YAMl file
    _write_test_yaml command=top

    # Run `play kube` in the background as it will wait for the service
    # container to exit.
    timeout --foreground -v --kill=10 60 \
        $PODMAN --syslog play kube --service-container=true --log-driver journald $TESTYAML &>/dev/null &

    # Wait for the container to be running
    container_a=$PODCTRNAME
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
    yaml_sha=$(sha256sum $TESTYAML)
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
    run_podman stop $PODCTRNAME
    _ensure_pod_state $PODNAME Exited
    _ensure_container_running $service_container false
    run_podman container inspect $service_container --format "{{.IsService}}"
    is "$output" "true"

    # Restart the pod, make sure the service is running again
    run_podman --syslog pod restart $PODNAME
    run_podman container inspect $service_container --format "{{.State.Running}}"
    is "$output" "true"

    # Check for an error when trying to remove the service container
    run_podman 125 container rm $service_container
    is "$output" "Error: container .* is the service container of pod(s) .* and cannot be removed without removing the pod(s)"
    run_podman 125 container rm --force $service_container
    is "$output" "Error: container .* is the service container of pod(s) .* and cannot be removed without removing the pod(s)"

    # Kill the pod and make sure the service is not running
    run_podman --syslog pod kill $PODNAME
    _ensure_container_running $service_container false

    run_podman network ls

    # Remove the pod and make sure the service is removed along with it
    run_podman --syslog pod rm $PODNAME
    run_podman 1 container exists $service_container
}

@test "podman kube --network" {
    _write_test_yaml command=top

    run_podman kube play --network host $TESTYAML
    is "$output" "Pod:.*" "podman kube play should work with --network host"

    run_podman pod inspect --format "{{.InfraConfig.HostNetwork}}" $PODNAME
    is "$output" "true" ".InfraConfig.HostNetwork"
    run_podman pod rm -t 0 -f $PODNAME

    if has_slirp4netns; then
        run_podman kube play --network slirp4netns:port_handler=slirp4netns $TESTYAML
        run_podman pod inspect --format {{.InfraContainerID}} "${lines[1]}"
        infraID="$output"
        run_podman container inspect --format "{{.HostConfig.NetworkMode}}" $infraID
        is "$output" "slirp4netns" "network mode slirp4netns is set for the container"
    fi

    run_podman pod rm -t 0 -f $PODNAME

    run_podman kube play --network none $TESTYAML
    run_podman pod inspect --format {{.InfraContainerID}} "${lines[1]}"
    infraID="$output"
    run_podman container inspect --format "{{.HostConfig.NetworkMode}}" $infraID
    is "$output" "none" "network mode none is set for the container"

    run_podman container exists $PODCTRNAME
    run_podman kube down $TESTYAML
    run_podman 1 container exists $PODCTRNAME
}

@test "podman kube play read-only" {
    YAML=$PODMAN_TMPDIR/test.yml

    podname="p-$(safename)"
    c1name="c1-$(safename)"
    c2name="c2-$(safename)"
    c3name="c3-$(safename)"

    # --restart=no is crucial: without that, the "podman wait" below
    # will spin for indeterminate time.
    run_podman create --pod new:$podname         --restart=no --name $c1name $IMAGE touch /testrw
    run_podman create --pod $podname --read-only --restart=no --name $c2name $IMAGE touch /testro
    run_podman create --pod $podname --read-only --restart=no --name $c3name $IMAGE sh -c "echo "#!echo hi" > /tmp/testtmp; chmod +x /tmp/test/tmp; /tmp/testtmp"

    # Generate and run from yaml. (The "cat" is for debugging failures)
    run_podman kube generate $podname -f $YAML
    cat $YAML
    run_podman kube play --replace $YAML

    # Wait for all containers and check their exit statuses
    run_podman wait ${podname}-${c1name} ${podname}-${c2name} ${podname}-${c3name}
    is "${lines[0]}" 0 "exit status: touch /file on read/write container"
    is "${lines[1]}" 1 "exit status: touch /file on read-only container"
    is "${lines[2]}" 0 "exit status: touch on /tmp is always ok, even on read-only container"

    # Confirm config settings
    run_podman container inspect --format '{{.HostConfig.ReadonlyRootfs}}' ${podname}-${c1name} ${podname}-${c2name} ${podname}-${c3name}
    is "${lines[0]}" "false"  "ReadonlyRootfs - container 1"
    is "${lines[1]}" "true"   "ReadonlyRootfs - container 2"
    is "${lines[2]}" "true"   "ReadonlyRootfs - container 3"

    # Clean up
    run_podman kube down - < $YAML
    run_podman 1 container exists ${podname}-${c1name}
    run_podman 1 container exists ${podname}-${c2name}
    run_podman 1 container exists ${podname}-${c3name}
}

@test "podman kube play read-only from containers.conf" {
    containersconf=$PODMAN_TMPDIR/containers.conf
    cat >$containersconf <<EOF
[containers]
read_only=true
EOF

    YAML=$PODMAN_TMPDIR/test.yml

    podname="p-$(safename)"
    c1name="c1-$(safename)"
    c2name="c2-$(safename)"
    c3name="c3-$(safename)"

    # --restart=no is crucial: without that, the "podman wait" below
    # will spin for indeterminate time.
    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman create --pod new:$podname --read-only=false --restart=no --name $c1name $IMAGE touch /testrw
    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman create --pod $podname                       --restart=no --name $c2name $IMAGE touch /testro
    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman create --pod $podname                       --restart=no --name $c3name $IMAGE touch /tmp/testtmp

    # Inspect settings in created containers
    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman container inspect --format '{{.HostConfig.ReadonlyRootfs}}' $c1name $c2name $c3name
    is "${lines[0]}" "false"  "ReadonlyRootfs - container 1, created"
    is "${lines[1]}" "true"   "ReadonlyRootfs - container 2, created"
    is "${lines[2]}" "true"   "ReadonlyRootfs - container 3, created"

    # Now generate and run kube.yaml on a machine without the defaults set
    CONTAINERS_CONF_OVERRIDE="$containersconf" run_podman kube generate $podname -f $YAML
    cat $YAML

    run_podman kube play --replace $YAML

    # Wait for all containers and check their exit statuses
    run_podman wait ${podname}-${c1name} ${podname}-${c2name} ${podname}-${c3name}
    is "${lines[0]}" 0 "exit status: touch /file on read/write container"
    is "${lines[1]}" 1 "exit status: touch /file on read-only container"
    is "${lines[2]}" 0 "exit status: touch on /tmp is always ok, even on read-only container"

    # Confirm settings again
    run_podman container inspect --format '{{.HostConfig.ReadonlyRootfs}}' ${podname}-${c1name} ${podname}-${c2name} ${podname}-${c3name}
    is "${lines[0]}" "false"  "ReadonlyRootfs - container 1, post-run"
    is "${lines[1]}" "true"   "ReadonlyRootfs - container 2, post-run"
    is "${lines[2]}" "true"   "ReadonlyRootfs - container 3, post-run"

    # Clean up
    run_podman kube down - < $YAML
    run_podman 1 container exists ${podname}-${c1name}
    run_podman 1 container exists ${podname}-${c2name}
    run_podman 1 container exists ${podname}-${c3name}
}

@test "podman play with user from image" {
    imgname="i-$(safename)"
    _write_test_yaml command=id image=$imgname

    cat > $PODMAN_TMPDIR/Containerfile << _EOF
from $IMAGE
USER bin
_EOF

    # Unset the PATH during build and make sure that all default env variables
    # are correctly set for the created container.
    run_podman build --unsetenv PATH -t $imgname $PODMAN_TMPDIR
    run_podman image inspect $imgname --format "{{.Config.Env}}"
    is "$output" "\[\]" "image does not set PATH - env is empty"

    run_podman play kube --start=false $TESTYAML
    run_podman inspect --format "{{ .Config.User }}" $PODCTRNAME
    is "$output" bin "expect container within pod to run as the bin user"
    run_podman inspect --format "{{ .Config.Env }}" $PODCTRNAME
    is "$output" ".*PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin.*" "expect PATH to be set"
    is "$output" ".*container=podman.*" "expect container to be set"

    run_podman pod rm -t 0 -f $PODNAME
    run_podman rmi -f $imgname
}

@test "podman play --build --context-dir" {
    skip_if_remote "--build is not supported in context remote"

    imgname="userimage-$(safename)"

    mkdir -p $PODMAN_TMPDIR/$imgname
    cat > $PODMAN_TMPDIR/$imgname/Containerfile << _EOF
from $IMAGE
USER bin
_EOF

    _write_test_yaml command=id image=quay.io/libpod/$imgname
    run_podman 125 play kube --build --start=false $TESTYAML
    assert "$output" =~ "initializing source docker://quay.io/libpod/$imgname:latest: reading manifest latest in "

    run_podman play kube --replace --context-dir=$PODMAN_TMPDIR --build --start=false $TESTYAML
    run_podman inspect --format "{{ .Config.User }}" $PODCTRNAME
    is "$output" bin "expect container within pod to run as the bin user"

    run_podman pod rm -t 0 -f $PODNAME
    run_podman rmi -f $imgname

    cd $PODMAN_TMPDIR
    run_podman play kube --replace --build --start=false $TESTYAML
    run_podman inspect --format "{{ .Config.User }}" $PODCTRNAME
    is "$output" bin "expect container within pod to run as the bin user"

    run_podman pod rm -t 0 -f $PODNAME
    run_podman rmi -f $imgname
}

# Occasionally a remnant storage container is left behind which causes
# podman play kube --replace to fail. This tests created a conflicting
# storage container name using buildah to make sure --replace, still
# functions properly by removing the storage container.
@test "podman kube play --replace external storage" {
    _write_test_yaml command="top"

    run_podman play kube $TESTYAML

    # Pod container exists
    run_podman ps -a --format '=={{.Names}}=={{.Status}}=='
    assert "$output" =~ "==${PODCTRNAME}==Up " "Pod container created"

    # Force removal of container, and confirm that it no longer exists
    run_podman rm --force -t0 $PODCTRNAME
    assert "$output" = "$PODCTRNAME" "Confirmation that container was removed"
    run_podman ps -a --external --format '=={{.Names}}=={{.Status}}=='
    assert "$output" !~ "$PODCTRNAME" "Pod container gone"

    # Create external container using buildah with same name
    buildah from --name $PODCTRNAME $IMAGE
    # Confirm that we now have an external (Storage) container by that name
    run_podman ps -a --external --format '=={{.Names}}=={{.Status}}=='
    assert "$output" =~ "==${PODCTRNAME}==Storage==" "Storage (external) container created"

    # --replace deletes the buildah container and replace it with new one.
    # Prior to #20025, this would fail with "container name is in use"
    run_podman play kube --replace $TESTYAML

    run_podman pod rm -t 0 -f $PODNAME
}

@test "podman kube --annotation" {
    _write_test_yaml command=/home/podman/pause

    RANDOMSTRING=$(random_string 15)
    ANNOTATION_WITH_COMMA="comma,$(random_string 5)"
    run_podman kube play --annotation "name=$RANDOMSTRING"  \
        --annotation "anno=$ANNOTATION_WITH_COMMA" $TESTYAML
    run_podman inspect --format "{{ .Config.Annotations }}" $PODCTRNAME
    is "$output" ".*name:$RANDOMSTRING" "Annotation should be added to pod"
    is "$output" ".*anno:$ANNOTATION_WITH_COMMA" "Annotation with comma should be added to pod"

    # invalid annotation
    run_podman 125 kube play --annotation "val" $TESTYAML
    assert "$output" == "Error: annotation \"val\" must include an '=' sign" "invalid annotation error"

    run_podman pod rm -t 0 -f $PODNAME
}

@test "podman play Yaml deprecated --no-trunc annotation" {
   skip "FIXME: I can't figure out what this test is supposed to do"
   RANDOMSTRING=$(random_string 65)

   _write_test_yaml "annotations=test: ${RANDOMSTRING}" command=id
   run_podman play kube --no-trunc - < $TESTYAML
}

@test "podman kube play - default log driver" {
    _write_test_yaml command=top
    # Get the default log driver
    run_podman info --format "{{.Host.LogDriver}}"
    default_driver=$output

    # Make sure that the default log driver is used
    run_podman kube play $TESTYAML
    run_podman inspect --format "{{.HostConfig.LogConfig.Type}}" $PODCTRNAME
    is "$output" "$default_driver" "play kube uses default log driver"

    run_podman kube down $TESTYAML
    run_podman 125 inspect $PODCTRNAME
    is "$output" ".*Error: no such object: \"$PODCTRNAME\""
}

@test "podman kube play - URL" {
    _write_test_yaml command=top

    echo READY > $PODMAN_TMPDIR/ready

    HOST_PORT=$(random_free_port)
    SERVER=http://127.0.0.1:$HOST_PORT

    serverctr="yamlserver-$(safename)"
    run_podman run -d --name $serverctr -p "$HOST_PORT:80" \
               -v $TESTYAML:/var/www/testpod.yaml:Z \
               -v $PODMAN_TMPDIR/ready:/var/www/ready:Z \
               -w /var/www \
               $IMAGE /bin/busybox-extras httpd -f -p 80

    wait_for_port 127.0.0.1 $HOST_PORT
    wait_for_command_output "curl -s -S $SERVER/ready" "READY"

    run_podman kube play $SERVER/testpod.yaml
    run_podman inspect $PODCTRNAME --format "{{.State.Running}}"
    is "$output" "true"
    run_podman kube down $SERVER/testpod.yaml
    run_podman 125 inspect $PODCTRNAME
    is "$output" ".*Error: no such object: \"$PODCTRNAME\""

    run_podman rm -f -t0 $serverctr
}

@test "podman play with init container" {
    _write_test_yaml command=
    cat >>$TESTYAML <<EOF
  - command:
    - ls
    - /dev/shm/test1
    image: $IMAGE
    name: ${CTRNAME}-test
  initContainers:
  - command:
    - touch
    - /dev/shm/test1
    image: $IMAGE
    name: ${CTRNAME}-init
EOF

    run_podman kube play $TESTYAML
    assert "$output" !~ "level=" "init containers should not generate logrus.Error"
    run_podman inspect --format "{{.State.ExitCode}}" ${PODNAME}-${CTRNAME}-test
    is "$output" "0" "init container should have created /dev/shm/test1"

    run_podman kube down $TESTYAML
}

@test "podman kube play - hostport" {
    HOST_PORT=$(random_free_port)
    _write_test_yaml
    cat >>$TESTYAML <<EOF
    - name: ${CTRNAME}-server
      image: $IMAGE
      ports:
        - name: hostp
          hostPort: $HOST_PORT
EOF

    run_podman kube play $TESTYAML
    run_podman pod inspect $PODNAME --format "{{.InfraConfig.PortBindings}}"
    assert "$output" = "map[$HOST_PORT/tcp:[{0.0.0.0 $HOST_PORT}]]"
    run_podman kube down $TESTYAML
}

@test "podman kube play - multi-pod YAML" {
    skip_if_remote "service containers only work locally"
    skip_if_journald_unavailable

    # Create the YAMl file, with two pods, each with one container
    podnamebase="p-$(safename)"
    ctrnamebase="c-$(safename)"
    for n in 1 2;do
        _write_test_yaml labels="app: $podnamebase-$n" name="$podnamebase-$n" ctrname="$ctrnamebase-$n" command=top

        # Separator between two yaml halves
        if [[ $n = 1 ]]; then
            echo "---" >>$TESTYAML
        fi
    done

    # Run `play kube` in the background as it will wait for the service
    # container to exit.
    timeout --foreground -v --kill=10 60 \
        $PODMAN play kube --service-container=true --log-driver journald $TESTYAML &>/dev/null &

    # The name of the service container is predictable: the first 12 characters
    # of the hash of the YAML file followed by the "-service" suffix
    yaml_sha=$(sha256sum $TESTYAML)
    service_container="${yaml_sha:0:12}-service"
    # Wait for the containers to be running
    container_1="${podnamebase}-1-${ctrnamebase}-1"
    container_2="${podnamebase}-2-${ctrnamebase}-2"
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
    run_podman pod stop ${podnamebase}-1 ${podnamebase}-2
    assert "$output" !~ "Stopping"
    _ensure_container_running $service_container false

    run_podman kube down $TESTYAML
}

@test "podman kube generate filetype" {
    YAML=$PODMAN_TMPDIR/test.yml

    podname="p-$(safename)"
    ctrname="c-$(safename)"
    volname="v-$(safename)"

    run_podman create --pod new:$podname \
               --security-opt label=level:s0:c1,c2 \
               --security-opt label=filetype:usr_t \
               -v $volname:/myvol --name $ctrname $IMAGE true
    run_podman kube generate $podname -f $YAML
    run cat $YAML
    is "$output" ".*filetype: usr_t" "Generated YAML file should contain filetype usr_t"
    run_podman pod rm --force $podname
    run_podman volume rm -t -1 $volname --force

    run_podman kube play $YAML
    if selinux_enabled; then
        run_podman inspect ${podname}-${ctrname} --format "{{ .MountLabel }}"
        is "$output" "system_u:object_r:usr_t:s0:c1,c2" "Generated container should use filetype usr_t"
        run_podman volume inspect $volname --format '{{ .Mountpoint }}'
        path=${output}
        run ls -Zd $path
        is "$output" "system_u:object_r:usr_t:s0 $path" "volume should be labeled with usr_t type"
    fi
    run_podman kube down $YAML
    run_podman volume rm $volname --force
}

# kube play --wait=true, where we clear up the created containers, pods, and volumes when a kill or sigterm is triggered
@test "podman kube play --wait with siginterrupt" {
    podname="p-$(safename)"
    ctrname="c-$(safename)"

    fname="$PODMAN_TMPDIR/play_kube_wait_$(random_string 6).yaml"
    echo "
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: $podname
spec:
  restartPolicy: Never
  containers:
    - name: $ctrname
      image: $IMAGE
      command:
      - top
" > $fname

    # force a timeout to happen so that the kube play command is killed
    # and expect the timeout code 124 to happen so that we can clean up
    local t0=$SECONDS
    PODMAN_TIMEOUT=2 run_podman 124 kube play --wait $fname
    local t1=$SECONDS
    local delta_t=$((t1 - t0))

    # Expectation (in seconds) of when we should time out. When running
    # parallel, allow 2 more seconds due to system load
    local expect=4
    if [[ -n "$PARALLEL_JOBSLOT" ]]; then
        expect=$((expect + 2))
    fi
    assert $delta_t -le $expect \
           "podman kube play did not get killed within $expect seconds"
    # Make sure we actually got SIGTERM and podman printed its message.
    assert "$output" =~ "Cleaning up containers, pods, and volumes" "kube play printed sigterm message"

    # there should be no containers running or created
    run_podman ps -aq
    assert "$output" !~ "$(safename)" "No containers created by this test"
}

@test "podman kube play --wait - wait for pod to exit" {
    podname="p-$(safename)"
    ctrname="c-$(safename)"

    fname="$PODMAN_TMPDIR/play_kube_wait_$(random_string 6).yaml"
    echo "
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: $podname
spec:
  restartPolicy: Never
  containers:
    - name: $ctrname
      image: $IMAGE
      command:
      - echo
      - hello
" > $fname

    run_podman kube play --wait $fname

    # debug to see what container is being left behind after the cleanup
    # there should be no containers running or created
    run_podman ps -a --noheading
    assert "$output" !~ "$(safename)" "No containers created by this test"
    run_podman pod ps
    assert "$output" !~ "$(safename)" "No pods created by this test"
}

@test "podman kube play with configmaps" {
    foovalue="foo-$(safename)"
    barvalue="bar-$(safename)"

    configmap_file=${PODMAN_TMPDIR}/play_kube_configmap_configmaps$(random_string 6),withcomma.yaml
    echo "
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: foo
data:
  value: $foovalue
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: bar
data:
  value: $barvalue
" > $configmap_file

    podname="p-$(safename)"
    ctrname="c-$(safename)"

    pod_file=${PODMAN_TMPDIR}/play_kube_configmap_pod$(random_string 6).yaml
    echo "
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: $podname
spec:
  restartPolicy: Never
  containers:
  - name: $ctrname
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
    run_podman wait $podname-$ctrname

    # systemd logs are unreliable; we may need to retry a few times
    # https://github.com/systemd/systemd/issues/28650
    local retries=10
    while [[ $retries -gt 0 ]]; do
        run_podman logs $podname-$ctrname
        test -n "$output" && break
        sleep 0.1
        retries=$((retries - 1))
    done
    assert "$retries" -gt 0 "Timed out waiting for podman logs"
    assert "$output" = "$foovalue:$barvalue" "output from podman logs"

    run_podman kube down $pod_file
}

@test "podman kube with --authfile=/tmp/bogus" {
    _write_test_yaml
    bogus=$PODMAN_TMPDIR/bogus-authfile

    run_podman 125 kube play --authfile=$bogus - < $TESTYAML
    is "$output" "Error: credential file is not accessible: faccessat $bogus: no such file or directory" \
           "$command should fail with not such file"
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

    ctr="c-$(safename)"
    ctrInPod="${ctr}-pod-${ctr}"

    run_podman create --restart never --name $ctr $IMAGE sh -c "touch /umask-test;stat -c '%a' /umask-test"
    run_podman kube generate -f $YAML $ctr
    CONTAINERS_CONF_OVERRIDE="$containersConf" run_podman kube play $YAML
    run_podman container inspect --format '{{ .Config.Umask }}' $ctrInPod
    is "${output}" "0472"
    # Confirm that umask actually takes effect. Might take a second or so.
    local retries=10
    while [[ $retries -gt 0 ]]; do
        run_podman logs $ctrInPod
        test -n "$output" && break
        sleep 0.1
        retries=$((retries - 1))
    done
    assert "$retries" -gt 0 "Timed out waiting for container output"
    assert "$output" = "204" "stat() on created file"

    run_podman kube down $YAML
    run_podman rm $ctr
}

@test "podman kube generate tmpfs on /tmp" {
    _write_test_yaml command=/home/podman/pause
    run_podman kube play $TESTYAML
    run_podman exec $PODCTRNAME sh -c "mount | grep /tmp"
    assert "$output" !~ "noexec" "mounts on /tmp should not be noexec"
    run_podman kube down $TESTYAML
}

@test "podman kube play - pull policy" {
    skip_if_remote "pull debug logs only work locally"

    _write_test_yaml command=true

    # Exploit a debug message to make sure the expected pull policy is used
    run_podman --debug kube play $TESTYAML
    assert "$output" =~ "Pulling image $IMAGE \(policy\: missing\)" "default pull policy is missing"
    run_podman kube down $TESTYAML

    local_image="localhost/i-$(safename):latest"
    run_podman tag $IMAGE $local_image
    rm $TESTYAML
    _write_test_yaml command=true image=$local_image

    run_podman --debug kube play $TESTYAML
    assert "$output" =~ "Pulling image $local_image \(policy\: newer\)" "pull policy is set to newhen pulling latest tag"
    run_podman kube down $TESTYAML

    run_podman rmi $local_image
}

@test "podman kube play healthcheck should wait initialDelaySeconds before updating status (healthy)" {
    podname="liveness-exec-$(safename)"
    ctrname="liveness-ctr-$(safename)"

    fname="$PODMAN_TMPDIR/play_kube_healthy_$(random_string 6).yaml"
    echo "
apiVersion: v1
kind: Pod
metadata:
  labels:
  name: $podname
spec:
  containers:
  - name: $ctrname
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
    ctrName="$podname-$ctrname"

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
}

@test "podman kube play healthcheck should wait initialDelaySeconds before updating status (unhealthy)" {
    podname="liveness-exec-$(safename)"
    ctrname="liveness-ctr-$(safename)"

    fname="$PODMAN_TMPDIR/play_kube_unhealthy_$(random_string 6).yaml"
    echo "
apiVersion: v1
kind: Pod
metadata:
  labels:
  name: $podname
spec:
  containers:
  - name: $ctrname
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
    ctrName="$podname-$ctrname"

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
}

@test "podman play --build private registry" {
    skip_if_remote "--build is not supported in context remote"

    local registry=localhost:${PODMAN_LOGIN_REGISTRY_PORT}
    local from_image=$registry/quadlet_image_test-$(safename):$(random_string)
    local authfile=$PODMAN_TMPDIR/authfile.json

    userimage=userimage-$(safename)
    mkdir -p $PODMAN_TMPDIR/$userimage
    cat > $PODMAN_TMPDIR/$userimage/Containerfile << _EOF
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

    _write_test_yaml command=id image=$userimage
    run_podman 125 play kube --build --start=false $TESTYAML
    assert "$output" "=~" \
        "Error: short-name resolution enforced but cannot prompt without a TTY|Resolving \"$userimage\" using unqualified-search registries" \
        "The error message does match any of the expected ones"

    run_podman play kube --replace --context-dir=$PODMAN_TMPDIR --tls-verify=false --authfile=$authfile --build --start=false $TESTYAML
    run_podman inspect --format "{{ .Config.User }}" $PODCTRNAME
    is "$output" bin "expect container within pod to run as the bin user"

    run_podman kube down $TESTYAML
    run_podman rmi -f $userimage $from_image
}

@test "podman play with automount volume" {
    imgname1="automount-img1-$(safename)"
    imgname2="automount-img2-$(safename)"
    podname="p-$(safename)"
    ctrname="c-$(safename)"
    ctrname_not_mounted="c-not-mounted-$(safename)"

    cat >$PODMAN_TMPDIR/Containerfile1 <<EOF
FROM $IMAGE
RUN mkdir /test1 /test_same && \
    touch /test1/a /test1/b /test1/c && \
    echo "I am from test1 image" > /test_same/hello_world
VOLUME /test1
VOLUME /test_same
EOF

    cat >$PODMAN_TMPDIR/Containerfile2 <<EOF
FROM $IMAGE
RUN mkdir /test2 /test_same && \
    touch /test2/asdf /test2/ejgre /test2/lteghe && \
    echo "I am from test2 image" > /test_same/hello_world
VOLUME /test2
VOLUME /test_same
EOF

    run_podman build -t $imgname1 -f $PODMAN_TMPDIR/Containerfile1
    run_podman build -t $imgname2 -f $PODMAN_TMPDIR/Containerfile2

    _write_test_yaml command=top name=$podname ctrname=$ctrname
    run_podman kube play --annotation "io.podman.annotations.kube.image.volumes.mount/$ctrname=$imgname1" $TESTYAML

    run_podman run --rm $imgname1 ls -x /test1
    assert "$output" = "a  b  c" "ls /test1 on image"
    run_out_test1="$output"
    run_podman exec $podname-$ctrname ls -x /test1
    assert "$output" = "$run_out_test1" "matching ls run/exec volume path test1"

    run_podman run --rm $imgname1 cat /test_same/hello_world
    assert "$output" = "I am from test1 image"  "cat /test_same/hello_world on image"
    run_out_hello_world="$output"
    run_podman exec $podname-$ctrname cat /test_same/hello_world
    assert "$output" = "$run_out_hello_world" "matching cat /test_same/hello_world volume path test_same"

    run_podman kube down $TESTYAML

    fname="/$PODMAN_TMPDIR/play_kube_wait_$(random_string 6).yaml"
    cat >$fname <<EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: test
  name: $podname
  annotations:
    io.podman.annotations.kube.image.volumes.mount/$ctrname: $imgname1;$imgname2
spec:
  restartPolicy: Never
  containers:
    - name: $ctrname
      image: $IMAGE
      command:
      - top
    - name: $ctrname_not_mounted
      image: $IMAGE
      command:
      - top
EOF

    run_podman kube play $fname

    run_podman exec "$podname-$ctrname" ls -x /test1
    assert "a  b  c" "ls /test1 inside container"

    run_podman exec "$podname-$ctrname" ls -x /test2
    assert "asdf    ejgre   lteghe" "ls /test2 inside container"

    run_podman exec "$podname-$ctrname" cat /test_same/hello_world
    assert "I am from test2 image" "cat /test_same/hello_world inside container"

    run_podman 1 exec "$podname-$ctrname" touch /test1/readonly
    assert "$output" =~ "Read-only file system" "image mounted as readonly"

    run_podman exec "$podname-$ctrname_not_mounted" ls /
    assert "$output" !~ "test" "No volume should be mounted in no-mount container"

    run_podman kube down $fname
    run_podman rmi $imgname1 $imgname2
}

@test "podman kube restore user namespace" {
    if ! is_rootless; then
        grep -E -q "^containers:" /etc/subuid || skip "no IDs allocated for user 'containers'"
    fi

    podname="p-userns-$(safename)"
    run_podman pod create --userns auto --name $podname
    run_podman create --pod $podname $IMAGE true

    run_podman pod inspect --format {{.InfraContainerID}} $podname
    infraID="$output"

    run_podman inspect --format '{{index .Config.Annotations "io.podman.annotations.userns"}}' $infraID
    assert "$output" == "auto" "user namespace should be kept"

    YAML=$PODMAN_TMPDIR/test.yml

    # Make sure the same setting is restored if the pod is recreated from the yaml
    run_podman kube generate $podname -f $YAML
    cat $YAML
    run_podman kube play --replace $YAML

    run_podman pod inspect --format {{.InfraContainerID}} $podname
    infraID="$output"

    run_podman inspect --format '{{index .Config.Annotations "io.podman.annotations.userns"}}' $infraID
    assert "$output" == "auto" "user namespace should be kept"

    run_podman pod rm -f $podname
}
