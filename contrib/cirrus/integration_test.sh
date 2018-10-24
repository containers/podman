#!/bin/bash

set -e
source $(dirname $0)/lib.sh

req_env_var "
GOSRC $GOSRC
OS_RELEASE_ID $OS_RELEASE_ID
OS_RELEASE_VER $OS_RELEASE_VER
"

make install.tools "BUILDTAGS=$BUILDTAGS"
make "BUILDTAGS=$BUILDTAGS"
make install PREFIX=/usr ETCDIR=/etc "BUILDTAGS=$BUILDTAGS"

cd $GOSRC/contrib/python/podman
# Run one test, over and over forever, recording runs, passes, and failures.
TESTNAME="test.test_containers"
CMPLTD=0
PASSED=0
FAILED=0

echo "."
echo ">>>>> Beginning continuous testing, only non-zero exits will be shown."
echo "."

BIGSTART=$(date +%s)

while true
do
    START=$(date +%s)
    set +e  # let was designed by a prehistoric moron
        # ooe.sh ./test/test_runner.sh -v $TESTNAME &> /tmp/test.log
        if ooe.sh ./test/test_runner.sh -v &> /tmp/test.log
        then
            let PASSED++
            rm -f /tmp/test.log
            NOW=$(date +%s)
            DIFF=$[NOW-START]
        else
            let FAILED++
            mv /tmp/test.log /tmp/failure_${CMPLTD}_test.log
            NOW=$(date +%s)
            DIFF=$[NOW-START]
            echo "-----systemd journal-----" >> /tmp/failure_${CMPLTD}_test.log
            journalctl -l --since "-$DIFF seconds" >> /tmp/failure_${CMPLTD}_test.log
        fi
        let CMPLTD++
        sync
        # Wipe out pagecache, dentries and inodes
        echo 3 > /proc/sys/vm/drop_caches
    set -e
    echo "$CMPLTD" > /tmp/test_runs_completed.txt
    echo "$FAILED" > /tmp/test_runs_failed.txt
    echo "$PASSED" > /tmp/test_runs_passed.txt

    MSG=" $CMPLTD runs. Of these, $PASSED have passed and $FAILED have failed"
    echo "."
    echo ">>>>> Completed $MSG. Time Delta $DIFF."
    echo "."

    # Give some status so we know it's running
    BIGDIFF=$[NOW-BIGSTART]
    if ((BIGDIFF >= 600))
    then
        BIGSTART=$(date +%s)
        ircmsg "cevich/jhonce, sir your cirrus-task-$CIRRUS_TASK_ID completed $MSG"
    fi
done

echo "somehow code got here but should not have"
