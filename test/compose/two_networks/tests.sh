# -*- bash -*-

ctr_name="two_networks_con1_1"
if [ "$TEST_FLAVOR" = "compose_v2" ]; then
    ctr_name="two_networks-con1-1"
fi
podman container inspect "$ctr_name" --format '{{len .NetworkSettings.Networks}}'
is "$output" "2" "$testname : Container is connected to both networks"
podman container inspect "$ctr_name" --format '{{.NetworkSettings.Networks}}'
like "$output" "two_networks_net1" "$testname : First network name exists"
like "$output" "two_networks_net2" "$testname : Second network name exists"
