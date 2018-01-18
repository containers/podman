#!/bin/sh
for i in $@; do
	filename=$(echo $i | sed 's/podman/docker/g')
	echo .so man1/$i > $filename
done
