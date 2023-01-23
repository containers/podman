#!/usr/bin/env sh
source ./helpers.bash

setup
echo_bold "List $NUM_CONTAINERS created containers"
create_containers
hyperfine --warmup 10 --runs $RUNS \
	"$ENGINE_A ps -a" \
	"$ENGINE_B ps -a"
