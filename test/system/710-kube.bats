#!/usr/bin/env bats   -*- bats -*-
#
# Test podman kube generate
#

load helpers

# capability drop list
capabilities='{"drop":["CAP_FOWNER","CAP_SETFCAP"]}'

# filter: convert yaml to json, because bash+yaml=madness
function yaml2json() {
    python3 -c 'import yaml
import json
import sys
json.dump(yaml.safe_load(sys.stdin), sys.stdout)'
}

###############################################################################
# BEGIN tests

@test "podman kube generate - usage message" {
    run_podman kube generate --help
    is "$output" ".*podman.* kube generate \[options\] {CONTAINER...|POD...|VOLUME...}"
    run_podman generate kube --help
    is "$output" ".*podman.* generate kube \[options\] {CONTAINER...|POD...|VOLUME...}"
}

@test "podman kube generate - container" {
    cname=c$(random_string 15)
    run_podman container create --cap-drop fowner --cap-drop setfcap --name $cname $IMAGE top
    run_podman kube generate $cname

    # As of #18542, we must never see this message again.
    assert "$output" !~ "Kubernetes only allows 63 characters"
    # Convert yaml to json, and dump to stdout (to help in case of errors)
    json=$(yaml2json <<<"$output")
    jq . <<<"$json"

    # What we expect to see. This is by necessity an incomplete list.
    # For instance, it does not include org.opencontainers.image.base.*
    # because sometimes we get that, sometimes we don't. No clue why.
    #
    # And, unfortunately, if new fields are added to the YAML, we won't
    # test those unless a developer remembers to add them here.
    #
    # Reasons for doing it this way, instead of straight-comparing yaml:
    #   1) the arbitrariness of the org.opencontainers.image.base annotations
    #   2) YAML order is nondeterministic, so on a pod with two containers
    #      (as in the pod test below) we cannot rely on cname1/cname2.
    expect="
apiVersion | =  | v1
kind       | =  | Pod

metadata.creationTimestamp | =~ | [0-9T:-]\\+Z
metadata.labels.app        | =  | ${cname}-pod
metadata.name              | =  | ${cname}-pod

spec.containers[0].command       | =  | [\"top\"]
spec.containers[0].image         | =  | $IMAGE
spec.containers[0].name          | =  | $cname

spec.containers[0].securityContext.capabilities  | =  | $capabilities

status                           | =  | null
"

    # Parse and check all those
    while read key op expect; do
        actual=$(jq -r -c ".$key" <<<"$json")
        assert "$actual" $op "$expect" ".$key"
    done < <(parse_table "$expect")

    run_podman rm $cname
}

@test "podman kube generate - pod" {
    local pname=p$(random_string 15)
    local cname1=c1$(random_string 15)
    local cname2=c2$(random_string 15)

    run_podman pod create --name $pname --publish 9999:8888

    # Needs at least one container. Error is slightly different between
    # regular and remote podman:
    #   regular: Error: pod ... only has...
    #   remote:  Error: generating YAML: pod ... only has...
    run_podman 125 kube generate $pname
    assert "$output" =~ "Error: .* only has an infra container"

    run_podman container create --cap-drop fowner --cap-drop setfcap --name $cname1 --pod $pname $IMAGE top
    run_podman container create --name $cname2 --pod $pname $IMAGE bottom
    run_podman kube generate $pname

    json=$(yaml2json <<<"$output")
    jq . <<<"$json"

    # See container test above for description of this table
    expect="
apiVersion | =  | v1
kind       | =  | Pod

metadata.creationTimestamp | =~ | [0-9T:-]\\+Z
metadata.labels.app        | =  | ${pname}
metadata.name              | =  | ${pname}

spec.hostname                              | =  | null

spec.containers[0].command                 | =  | [\"top\"]
spec.containers[0].image                   | =  | $IMAGE
spec.containers[0].name                    | =  | $cname1
spec.containers[0].ports[0].containerPort  | =  | 8888
spec.containers[0].ports[0].hostPort       | =  | 9999
spec.containers[0].resources               | =  | null

spec.containers[1].command                 | =  | [\"bottom\"]
spec.containers[1].image                   | =  | $IMAGE
spec.containers[1].name                    | =  | $cname2
spec.containers[1].ports                   | =  | null
spec.containers[1].resources               | =  | null

spec.containers[0].securityContext.capabilities  | =  | $capabilities

status  | =  | null
"

    while read key op expect; do
        actual=$(jq -r -c ".$key" <<<"$json")
        assert "$actual" $op "$expect" ".$key"
    done < <(parse_table "$expect")

    run_podman rm $cname1 $cname2
    run_podman pod rm $pname
    run_podman rmi $(pause_image)
}

@test "podman kube generate - deployment" {
    skip_if_remote "containersconf needs to be set on server side"
    local pname=p$(random_string 15)
    local cname1=c1$(random_string 15)
    local cname2=c2$(random_string 15)

    run_podman pod create --name $pname
    run_podman container create --name $cname1 --pod $pname $IMAGE top
    run_podman container create --name $cname2 --pod $pname $IMAGE bottom

    containersconf=$PODMAN_TMPDIR/containers.conf
    cat >$containersconf <<EOF
[engine]
kube_generate_type="deployment"
EOF
    CONTAINERS_CONF_OVERRIDE=$containersconf run_podman kube generate $pname

    json=$(yaml2json <<<"$output")
    # For debugging purposes in the event we regress, we can see the generate output to know what went wrong
    jq . <<<"$json"

    # See container test above for description of this table
    expect="
apiVersion | =  | apps/v1
kind       | =  | Deployment

metadata.creationTimestamp | =~ | [0-9T:-]\\+Z
metadata.labels.app        | =  | ${pname}
metadata.name              | =  | ${pname}-deployment
"

    while read key op expect; do
        actual=$(jq -r -c ".$key" <<<"$json")
        assert "$actual" $op "$expect" ".$key"
    done < <(parse_table "$expect")

    run_podman rm $cname1 $cname2
    run_podman pod rm $pname
    run_podman rmi $(pause_image)
}

# vim: filetype=sh
