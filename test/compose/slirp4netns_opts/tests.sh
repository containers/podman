# -*- bash -*-

output="$(cat $OUTFILE)"
expected="teststring"

is "$output" "$expected" "$testname : nc received teststring"
