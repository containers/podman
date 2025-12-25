# -*- bash -*-

expected="teststring"

# Reading from the nc socket is flaky because docker-compose only starts
# the containers. We cannot know at this point if the container did already
# send the message. Give the container 5 seconds time to send the message
# to prevent flakes.
container_timeout=5
while [ $container_timeout -gt 0 ]; do
    output="$(< $OUTFILE)"
    if [ -n "$output" ]; then
        break
    fi
    sleep 1
    container_timeout=$(($container_timeout - 1))
done

is "$output" "$expected" "$testname : nc received teststring"
