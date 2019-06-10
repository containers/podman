#!/bin/bash
#
# regression tests for helpers.bash
#
# Some of those helper functions are fragile, and we don't want to break
# anything if we have to mess with them.
#

source $(dirname $0)/helpers.bash

die() {
    echo "$(basename $0): $*" >&2
    exit 1
}

# Iterator and return code; updated in check_result()
testnum=0
rc=0

###############################################################################
# BEGIN test the parse_table helper

function check_result {
    testnum=$(expr $testnum + 1)
    if [ "$1" = "$2" ]; then
        echo "ok $testnum $3 = $1"
    else
        echo "not ok $testnum $3"
        echo "#  expected: $2"
        echo "#    actual: $1"
        rc=1
    fi
}

# IMPORTANT NOTE: you have to do
#      this: while ... done < <(parse_table)
#   and not: parse_table | while read ...
#
# ...because piping to 'while' makes it a subshell, hence testnum and rc
# will not be updated.
#
while read x y z; do
    check_result "$x" "a" "parse_table simple: column 1"
    check_result "$y" "b" "parse_table simple: column 2"
    check_result "$z" "c" "parse_table simple: column 3"
done < <(parse_table "a | b | c")

# More complicated example, with spaces
while read x y z; do
    check_result "$x" "a b"   "parse_table with spaces: column 1"
    check_result "$y" "c d"   "parse_table with spaces: column 2"
    check_result "$z" "e f g" "parse_table with spaces: column 3"
done < <(parse_table "a b | c d | e f g")

# Multi-row, with spaces and with blank lines
table="
a     | b   | c d e
d e f | g h | i j
"
declare -A expect=(
    [0,0]="a"
    [0,1]="b"
    [0,2]="c d e"
    [1,0]="d e f"
    [1,1]="g h"
    [1,2]="i j"
)
row=0
while read x y z;do
    check_result "$x" "${expect[$row,0]}" "parse_table multi_row[$row,0]"
    check_result "$y" "${expect[$row,1]}" "parse_table multi_row[$row,1]"
    check_result "$z" "${expect[$row,2]}" "parse_table multi_row[$row,2]"
    row=$(expr $row + 1)
done < <(parse_table "$table")

# Backslash handling. The first element should have none, the second some
while read x y;do
    check_result "$x" '[0-9]{2}'    "backslash test - no backslashes"
    check_result "$y" '[0-9]\{3\}'  "backslash test - one backslash each"
done < <(parse_table "[0-9]{2}  | [0-9]\\\{3\\\}")

# Empty strings. I wish we could convert those to real empty strings.
while read x y z; do
    check_result "$x" "''" "empty string - left-hand"
    check_result "$y" "''" "empty string - middle"
    check_result "$z" "''" "empty string - right"
done < <(parse_table " | |")

# Quotes
while read x y z;do
    check_result "$x" "a 'b c'"     "single quotes"
    check_result "$y" "d \"e f\" g" "double quotes"
    check_result "$z" "h"           "no quotes"

    # FIXME FIXME FIXME: this is the only way I can find to get bash-like
    # splitting of tokens. It really should be done inside parse_table
    # but I can't find any way of doing so. If you can find a way, please
    # update this test and any BATS tests that rely on quoting.
    eval set "$x"
    check_result "$1" "a"     "single quotes - token split - 1"
    check_result "$2" "b c"   "single quotes - token split - 2"
    check_result "$3" ""      "single quotes - token split - 3"

    eval set "$y"
    check_result "$1" "d"     "double quotes - token split - 1"
    check_result "$2" "e f"   "double quotes - token split - 2"
    check_result "$3" "g"     "double quotes - token split - 3"
done < <(parse_table "a 'b c' | d \"e f\" g | h")

# END   test the parse_table helper
###############################################################################
# BEGIN dprint

function dprint_test_1() {
    dprint "$*"
}

# parse_table works, might as well use it
#
#  <value of PODMAN_TEST_DEBUG> | <blank for no msg, - for msg> | <desc>
#
table="
                           |   | debug unset
dprint_test                | - | substring match
dprint_test_1              | - | exact match
dprint_test_10             |   | caller name mismatch
xxx yyy zzz                |   | multiple callers, no match
dprint_test_1 xxx yyy zzz  | - | multiple callers, match at start
xxx dprint_test_1 yyy zzz  | - | multiple callers, match in middle
xxx yyy zzz dprint_test_1  | - | multiple callers, match at end
"
while read var expect name; do
    random_string=$(random_string 20)
    PODMAN_TEST_DEBUG="$var" result=$(dprint_test_1 "$random_string" 3>&1)
    expect_full=""
    if [ -n "$expect" -a "$expect" != "''" ]; then
        expect_full="# dprint_test_1() : $random_string"
    fi
    check_result "$result" "$expect_full" "DEBUG='$var' - $name"
done < <(parse_table "$table")

# END   dprint
###############################################################################

exit $rc
