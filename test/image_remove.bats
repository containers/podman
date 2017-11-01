#!/usr/bin/env bats

load helpers

IMAGE=docker.io/kubernetes/pause

function teardown() {
	cleanup_test
}

@test "image remove with multiple names, by name" {
	start_crio "" "" --no-pause-image
	# Pull the image, giving it one name.
	run crioctl image pull "$IMAGE"
	echo "$output"
	[ "$status" -eq 0 ]
	# Add a second name to the image.
	run "$COPYIMG_BINARY" --root "$TESTDIR/crio" $STORAGE_OPTIONS --runroot "$TESTDIR/crio-run" --image-name="$IMAGE":latest --add-name="$IMAGE":othertag --signature-policy="$INTEGRATION_ROOT"/policy.json
	echo "$output"
	[ "$status" -eq 0 ]
	# Get the list of image names and IDs.
	run crioctl image list
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	# Cycle through each name, removing it by name.  The image that we assigned a second
	# name to should still be around when we get to removing its second name.
	grep ^Tag: <<< "$output" | while read -r header tag ; do
		run crioctl image remove --id "$tag"
		echo "$output"
		[ "$status" -eq 0 ]
	done
	# List all images and their names.  There should be none now.
	run crioctl image list --quiet
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" = "" ]
	printf '%s\n' "$output" | while IFS= read -r id; do
		echo "$id"
	done
	# All done.
	cleanup_images
	stop_crio
}

@test "image remove with multiple names, by ID" {
	start_crio "" "" --no-pause-image
	# Pull the image, giving it one name.
	run crioctl image pull "$IMAGE"
	echo "$output"
	[ "$status" -eq 0 ]
	# Add a second name to the image.
	run "$COPYIMG_BINARY" --root "$TESTDIR/crio" $STORAGE_OPTIONS --runroot "$TESTDIR/crio-run" --image-name="$IMAGE":latest --add-name="$IMAGE":othertag --signature-policy="$INTEGRATION_ROOT"/policy.json
	echo "$output"
	[ "$status" -eq 0 ]
	# Get the image ID of the image we just saved.
	run crioctl image status --id="$IMAGE"
	echo "$output"
	[ "$status" -eq 0 ]
	[ "$output" != "" ]
	# Try to remove the image using its ID.  That should succeed because removing by ID always works.
	grep ^ID: <<< "$output" | while read -r header id ; do
		run crioctl image remove --id "$id"
		echo "$output"
		[ "$status" -eq 0 ]
	done
	# The image should be gone.
	run crioctl image status --id="$IMAGE"
	echo "$output"
	[ "$status" -ne 0 ]
	# All done.
	cleanup_images
	stop_crio
}
