#!/bin/sh
for i in $@; do
    b=$(basename $i)
    filename=$(echo $i | sed 's/podman/docker/g')
    echo .so man1/$b > $filename
done
