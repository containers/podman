#!/bin/bash
#
# tests for lib.sh
#

# To ensure consistent sorting
export LC_ALL=C

###############################################################################
# BEGIN code to define a clean safe environment

# Envariables which we should keep; anything else, we toss.
declare -a keep_env_list=(IFS HOME PATH SECRET_ENV_RE
                          PASSTHROUGH_ENV_EXACT
                          PASSTHROUGH_ENV_ATSTART
                          PASSTHROUGH_ENV_ANYWHERE
                          PASSTHROUGH_ENV_RE
                          TMPDIR tmpdir keep_env rc_file testnum_file)
declare -A keep_env
for i in "${keep_env_list[@]}"; do
    keep_env[$i]=1
done

# END   code to define a clean safe environment
###############################################################################
# BEGIN test scaffolding

tmpdir=$(mktemp --tmpdir --directory lib-sh-tests.XXXXXXX)
# shellcheck disable=SC2154
trap 'status=$?; rm -rf $tmpdir;exit $status' 0

# Needed by lib.sh, but we don't actually need anything in it
touch "$tmpdir"/common_lib.sh

# Iterator and return code. Because some tests run in subshells (to avoid
# namespace pollution), variables aren't preserved. Use files to track them.
testnum_file=$tmpdir/testnum
rc_file=$tmpdir/rc

echo 0 >"$testnum_file"
echo 0 >"$rc_file"

# Helper function: runs passthrough_envars(), compares against expectations
function check_passthrough {
    testnum=$(< "$testnum_file")
    testnum=$((testnum + 1))
    echo $testnum > "$testnum_file"

    # shellcheck disable=SC2046,SC2005,SC2116
    actual="$(echo $(passthrough_envars))"

    if [[ "$actual" = "$1" ]]; then
        # Multi-level echo flattens newlines, makes success messages readable
        # shellcheck disable=SC2046,SC2005,SC2116
        echo $(echo "ok $testnum $2")
    else
        echo "not ok $testnum $2"
        echo "#  expected: $1"
        echo "#    actual: $actual"
        echo 1 >| "$rc_file"
    fi
}

# END   test scaffolding
###############################################################################

# vars and a function needed by lib.sh
# shellcheck disable=SC2034
{
    AUTOMATION_LIB_PATH=$tmpdir
    CIRRUS_BASE_SHA=x
    CIRRUS_TAG=x
    function warn() {
        :
    }
    # shellcheck disable=all
    source $(dirname "$0")/lib.sh
}

# Our environment is now super-polluted. Clean it up, preserving critical env.
while read -r v;do
      if [[ -z "${keep_env[$v]}" ]]; then
          unset "$v" 2>/dev/null
      fi
done < <(compgen -A variable)

# begin actual tests

check_passthrough "" "with empty environment"

#
# Now set all sorts of secrets, which should be excluded
#
# shellcheck disable=SC2034
{
    ACCOUNT_ABC=1
    ABC_ACCOUNT=1
    ABC_ACCOUNT_DEF=1
    GCEFOO=1
    GCPBAR=1
    SSH123=1
    NOTSSH=1
    SSH=1
    PASSWORD=1
    MYSECRET=1
    SECRET2=1
    TOKEN=1
    check_passthrough "" "secrets are filtered"
}

# These are passed through only when they match EXACTLY.
readarray -d '|' -t pt_exact <<<"$PASSTHROUGH_ENV_EXACT"
# shellcheck disable=SC2048
for s in ${pt_exact[*]}; do
    # Run inside a subshell, to avoid cluttering environment
    (
        eval "${s}=1"             # This is the only one that should be passed
        eval "a${s}=1"
        eval "${s}z=1"
        eval "YYY_${s}_YYY=1"
        eval "ZZZ_${s}=1"
        eval "${s}_ZZZ=1"

        # Only the exact match should be passed
        check_passthrough "$s" "exact match only: $s"
    )
done

# These are passed through only when they match AT THE BEGINNING.
#
# Also, we run this _entire_ test inside a subshell, cluttering the
# environment, so we're testing that passthrough_envars can handle
# and return long lists of unrelated matches. Kind of a pointless
# test, there's not really any imaginable way that could fail.
(
    # Inside the subshell. Start with null expectations.
    expect=

    # WARNING! $PASSTHROUGH_ENV_ATSTART must be in alphabetical order,
    # because passthrough_envars always returns a sorted list and (see
    # subshell comments above) we're incrementally adding to our env.
    readarray -d '|' -t pt_atstart <<<"$PASSTHROUGH_ENV_ATSTART"
    # shellcheck disable=SC2048
    for s in ${pt_atstart[*]}; do
        eval "${s}=1"
        eval "${s}123=1"
        eval "NOPE_${s}=1"
        eval "NOR_${s}_EITHER=1"

        if [[ -n "$expect" ]]; then
            expect+=" "
        fi
        expect+="$s ${s}123"

        check_passthrough "$expect" "at start only: $s"
    done
)

# These are passed through if they match ANYWHERE IN THE NAME
readarray -d '|' -t pt_anywhere <<<"$PASSTHROUGH_ENV_ANYWHERE"
# shellcheck disable=SC2048
for s in ${pt_anywhere[*]}; do
    (
        eval "${s}=1"
        eval "${s}z=1"
        eval "z${s}=1"
        eval "z${s}z=1"

        check_passthrough "${s} ${s}z z${s} z${s}z" "anywhere: $s"
    )
done

# And, to guard against null runs of the above loops, hardcoded tests of each:
# shellcheck disable=SC2034
(
    CI=1
    CI_FOO=1
    CIRRUS_BAR=1
    GOPATH=gopath
    GOPATH_NOT=not
    ROOTLESS_USER=rootless
    ZZZ_NAME=1

    check_passthrough "CI CIRRUS_BAR CI_FOO GOPATH ROOTLESS_USER ZZZ_NAME" \
                      "final handcrafted sanity check"
)

# Final check
check_passthrough "" "Environment remains unpolluted at end"

# Done
# shellcheck disable=all
exit $(<"$rc_file")
