#!/usr/bin/env sh
source ./helpers.bash

tmp=$(mktemp -d)

file_a=$(mktemp -p $tmp --suffix '.file_a')
file_b=$(mktemp -p $tmp --suffix '.file_b')
dd if=/dev/zero of=$file_a bs=1024 count=1024 status=none
dd if=/dev/zero of=$file_b bs=1024 count=1024 status=none

# The create command
volume_name="bz-2162111"
container_name="bz-2162111"
container_cmd="--name $container_name \
	--stop-timeout=0 \
	--network-alias alias_a \
	--network-alias alias_b \
	-v /dev/log:/dev/log:rw,z \
	-v $volume_name:/var/core:rw,z \
	-v $file_a:/home/file_a:rw \
	-v $file_b:/home/file_b:rw \
	--cap-drop=ALL \
	--ulimit nofile=1024:2048 \
	$IMAGE true"

# Script to clean up before each benchmark below
prepare_sh=$(mktemp -p $tmp --suffix '.prepare.sh')
cat >$prepare_sh <<EOF
\$ENGINE rm -f \$(\$ENGINE ps -aq)  > /dev/null
\$ENGINE volume prune -f            > /dev/null
\$ENGINE volume create $volume_name > /dev/null
EOF
echo_bold "Prepare script: $prepare_sh"

# Script to create container below
create_sh=$(mktemp -p $tmp --suffix '.create.sh')
cat >$create_sh <<EOF
\$ENGINE create $container_cmd > /dev/null
EOF
echo_bold "Create script: $prepare_sh"

echo ""
echo "----------------------------------------------------"
echo
echo_bold "Create $NUM_CONTAINERS containers"
hyperfine --warmup 10 --runs $RUNS \
	--prepare "ENGINE=$ENGINE_A sh $prepare_sh" \
	--prepare "ENGINE=$ENGINE_B sh $prepare_sh" \
	"$ENGINE_A create $container_cmd" \
	"$ENGINE_B create $container_cmd"

echo ""
echo "----------------------------------------------------"
echo
echo_bold "Start $NUM_CONTAINERS containers"
hyperfine --warmup 10 --runs $RUNS \
	--prepare "ENGINE=$ENGINE_A sh $prepare_sh; ENGINE=$ENGINE_A sh $create_sh" \
	--prepare "ENGINE=$ENGINE_B sh $prepare_sh; ENGINE=$ENGINE_B sh $create_sh" \
	"$ENGINE_A start $container_name" \
	"$ENGINE_B start $container_name"

echo ""
echo "----------------------------------------------------"
echo
echo_bold "Remove $NUM_CONTAINERS containers"
hyperfine --warmup 10 --runs $RUNS \
	--prepare "ENGINE=$ENGINE_A sh $prepare_sh; ENGINE=$ENGINE_A sh $create_sh" \
	--prepare "ENGINE=$ENGINE_B sh $prepare_sh; ENGINE=$ENGINE_B sh $create_sh" \
	"$ENGINE_A rm -f $container_name" \
	"$ENGINE_B rm -f $container_name"
