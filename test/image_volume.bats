#!/usr/bin/env bats

load helpers

function teardown() {
	cleanup_test
}

@test "image volume ignore" {
	IMAGE_VOLUMES=ignore start_crio
	run crioctl pod run --config "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	image_volume_config=$(cat "$TESTDATA"/container_config.json | python -c 'import json,sys;obj=json.load(sys.stdin);obj["image"]["image"] = "mrunalp/image-volume-test"; obj["command"] = ["/bin/sleep", "600"]; json.dump(obj, sys.stdout)')
	echo "$image_volume_config" > "$TESTDIR"/container_image_volume.json
	run crioctl ctr create --config "$TESTDIR"/container_image_volume.json --pod "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crioctl ctr start --id "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl ctr execsync --id "$ctr_id" ls /imagevolume
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "Exit code: 1" ]]
	[[ "$output" =~ "ls: /imagevolume: No such file or directory" ]]
	run crioctl pod stop --id "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl pod remove --id "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "image volume bind" {
	IMAGE_VOLUMES=bind start_crio
	run crioctl pod run --config "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	image_volume_config=$(cat "$TESTDATA"/container_config.json | python -c 'import json,sys;obj=json.load(sys.stdin);obj["image"]["image"] = "mrunalp/image-volume-test"; obj["command"] = ["/bin/sleep", "600"]; json.dump(obj, sys.stdout)')
	echo "$image_volume_config" > "$TESTDIR"/container_image_volume.json
	run crioctl ctr create --config "$TESTDIR"/container_image_volume.json --pod "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"
	run crioctl ctr start --id "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl ctr execsync --id "$ctr_id" touch /imagevolume/test_file
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "Exit code: 0" ]]
	run crioctl pod stop --id "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl pod remove --id "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	cleanup_ctrs
	cleanup_pods
	stop_crio
}
