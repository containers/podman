#!/usr/bin/env bats

load helpers

IMAGE=kubernetes/pause
SIGNED_IMAGE=registry.access.redhat.com/rhel7-atomic:latest
UNSIGNED_IMAGE=docker.io/library/hello-world:latest

function teardown() {
	cleanup_test
}

@test "run container in pod with image ID" {
	start_crio
	run crioctl pod run --config "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"
	sed -e "s/%VALUE%/$REDIS_IMAGEID/g" "$TESTDATA"/container_config_by_imageid.json > "$TESTDIR"/ctr_by_imageid.json
	run crioctl ctr create --config "$TESTDIR"/ctr_by_imageid.json --pod "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "container status return image:tag if created by image ID" {
	start_crio

	run crioctl pod run --config "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"

	sed -e "s/%VALUE%/$REDIS_IMAGEID/g" "$TESTDATA"/container_config_by_imageid.json > "$TESTDIR"/ctr_by_imageid.json

	run crioctl ctr create --config "$TESTDIR"/ctr_by_imageid.json --pod "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"

	run crioctl ctr status --id "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "Image: redis:alpine" ]]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "container status return image@digest if created by image ID" {
	start_crio

	run crioctl pod run --config "$TESTDATA"/sandbox_config.json
	echo "$output"
	[ "$status" -eq 0 ]
	pod_id="$output"

	sed -e "s/%VALUE%/$REDIS_IMAGEID_DIGESTED/g" "$TESTDATA"/container_config_by_imageid.json > "$TESTDIR"/ctr_by_imageid.json

	run crioctl ctr create --config "$TESTDIR"/ctr_by_imageid.json --pod "$pod_id"
	echo "$output"
	[ "$status" -eq 0 ]
	ctr_id="$output"

	run crioctl ctr status --id "$ctr_id"
	echo "$output"
	[ "$status" -eq 0 ]
	[[ "$output" =~ "ImageRef: redis@sha256:03789f402b2ecfb98184bf128d180f398f81c63364948ff1454583b02442f73b" ]]

	cleanup_ctrs
	cleanup_pods
	stop_crio
}

@test "image pull and list" {
	start_crio "" "" --no-pause-image
	run crioctl image pull "$IMAGE"
	echo "$output"
	[ "$status" -eq 0 ]

	run crioctl image list --quiet "$IMAGE"
	[ "$status" -eq 0 ]
	echo "$output"
	[ "$output" != "" ]
	imageid="$output"

	run crioctl image list --quiet @"$imageid"
	[ "$status" -eq 0 ]
	echo "$output"
	[ "$output" != "" ]

	run crioctl image list --quiet "$imageid"
	[ "$status" -eq 0 ]
	echo "$output"
	[ "$output" != "" ]
	cleanup_images
	stop_crio
}

@test "image pull with signature" {
	start_crio "" "" --no-pause-image
	run crioctl image pull "$SIGNED_IMAGE"
	echo "$output"
	[ "$status" -eq 0 ]
	cleanup_images
	stop_crio
}

@test "image pull without signature" {
	start_crio "" "" --no-pause-image
	run crioctl image pull "$UNSIGNED_IMAGE"
	echo "$output"
	[ "$status" -ne 0 ]
	cleanup_images
	stop_crio
}

@test "image pull and list by tag and ID" {
	start_crio "" "" --no-pause-image
	run crioctl image pull "$IMAGE:go"
	echo "$output"
	[ "$status" -eq 0 ]

	run crioctl image list --quiet "$IMAGE:go"
	[ "$status" -eq 0 ]
	echo "$output"
	[ "$output" != "" ]
	imageid="$output"

	run crioctl image list --quiet @"$imageid"
	[ "$status" -eq 0 ]
	echo "$output"
	[ "$output" != "" ]

	run crioctl image list --quiet "$imageid"
	[ "$status" -eq 0 ]
	echo "$output"
	[ "$output" != "" ]
	cleanup_images
	stop_crio
}

@test "image pull and list by digest and ID" {
	start_crio "" "" --no-pause-image
	run crioctl image pull nginx@sha256:33eb1ed1e802d4f71e52421f56af028cdf12bb3bfff5affeaf5bf0e328ffa1bc
	echo "$output"
	[ "$status" -eq 0 ]

	run crioctl image list --quiet nginx@sha256:33eb1ed1e802d4f71e52421f56af028cdf12bb3bfff5affeaf5bf0e328ffa1bc
	[ "$status" -eq 0 ]
	echo "$output"
	[ "$output" != "" ]
	imageid="$output"

	run crioctl image list --quiet @"$imageid"
	[ "$status" -eq 0 ]
	echo "$output"
	[ "$output" != "" ]

	run crioctl image list --quiet "$imageid"
	[ "$status" -eq 0 ]
	echo "$output"
	[ "$output" != "" ]

	cleanup_images
	stop_crio
}

@test "image list with filter" {
	start_crio "" "" --no-pause-image
	run crioctl image pull "$IMAGE"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl image list --quiet "$IMAGE"
	echo "$output"
	[ "$status" -eq 0 ]
	printf '%s\n' "$output" | while IFS= read -r id; do
		run crioctl image remove --id "$id"
		echo "$output"
		[ "$status" -eq 0 ]
	done
	run crioctl image list --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	printf '%s\n' "$output" | while IFS= read -r id; do
		echo "$id"
		status=1
	done
	cleanup_images
	stop_crio
}

@test "image list/remove" {
	start_crio "" "" --no-pause-image
	run crioctl image pull "$IMAGE"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl image list --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	printf '%s\n' "$output" | while IFS= read -r id; do
		run crioctl image remove --id "$id"
		echo "$output"
		[ "$status" -eq 0 ]
	done
	run crioctl image list --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" = "" ]
	printf '%s\n' "$output" | while IFS= read -r id; do
		echo "$id"
		status=1
	done
	cleanup_images
	stop_crio
}

@test "image status/remove" {
	start_crio "" "" --no-pause-image
	run crioctl image pull "$IMAGE"
	echo "$output"
	[ "$status" -eq 0 ]
	run crioctl image list --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	printf '%s\n' "$output" | while IFS= read -r id; do
		run crioctl image status --id "$id"
		echo "$output"
		[ "$status" -eq 0 ]
		[ "$output" != "" ]
		run crioctl image remove --id "$id"
		echo "$output"
		[ "$status" -eq 0 ]
	done
	run crioctl image list --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" = "" ]
	printf '%s\n' "$output" | while IFS= read -r id; do
		echo "$id"
		status=1
	done
	cleanup_images
	stop_crio
}
