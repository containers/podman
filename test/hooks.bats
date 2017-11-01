#!/usr/bin/env bats

load helpers

function teardown() {
	cleanup_test
}

cp hooks/checkhook.sh ${HOOKSDIR}
sed "s|HOOKSDIR|${HOOKSDIR}|" hooks/checkhook.json > ${HOOKSDIR}/checkhook.json

@test "pod test hooks" {
	rm -f /run/hookscheck
	start_crio
	run crioctl pod run --config "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	run crioctl ctr create --config "$TESTDATA"/container_redis.json --pod "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crioctl ctr start --id "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl pod stop --id "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl pod remove --id "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run cat /run/hookscheck
	echo "$output"
	[ "$status" -eq 0 ]
	cleanup_ctrs
	cleanup_pods
	stop_crio
}
