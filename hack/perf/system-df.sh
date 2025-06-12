#!/usr/bin/env sh
source ./helpers.bash

setup
echo_bold "List $NUM_CONTAINERS created containers"
create_containers
hyperfine --warmup 10 --runs $RUNS \
    "$ENGINE_A system df" \
    "$ENGINE_B system df"

# Clean up
$ENGINE_A system prune -f >>/dev/null
$ENGINE_B system prune -f >>/dev/null
