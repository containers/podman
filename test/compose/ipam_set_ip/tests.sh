# -*- bash -*-

podman container inspect ipam_set_ip_test_1 --format '{{ .NetworkSettings.Networks.ipam_set_ip_net1.IPAddress }}'
like "$output" "10.123.0.253" "$testname : ip address is set"
