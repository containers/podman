# -*- bash -*-

output="$(cat $OUTFILE)"
expected="teststring"

# Reading from the nc socket is flaky because docker-compose only starts
# the containers. We cannot know at this point if the container did already
# send the message. Give the container 5 seconds time to send the message
# to prevent flakes.
local _timeout=5
while [ $_timeout -gt 0 ]; do
    if [ -n "$output" ]; then
        break
    fi
    sleep 1
    _timeout=$(($_timeout - 1))
    output="$(cat $OUTFILE)"
done

is "$output" "$expected" "$testname : nc received teststring"
