Quick overview of podman system tests. The idea is to use BATS,
but with a framework for making it easy to add new tests and to
debug failures.

Quick Start
===========

Look at [030-run.bats](030-run.bats) for a simple but packed example.
This introduces the basic set of helper functions:

* `setup` (implicit) - resets container storage so there's
one and only one (standard) image, and no running containers.

* `parse_table` - you can define tables of inputs and expected results,
then read those in a `while` loop. This makes it easy to add new tests.
Because bash is not a programming language, the caller of `parse_table`
sometimes needs to massage the returned values; `015-run.bats` offers
examples of how to deal with the more typical such issues.

* `run_podman` - runs command defined in `$PODMAN` (default: 'podman'
but could also be 'podman-remote'), with a timeout. Checks its exit status.

* `is` - compare actual vs expected output. Emits a useful diagnostic
on failure.

* `random_string` - returns a pseudorandom alphanumeric string

Test files are of the form `NNN-name.bats` where NNN is a three-digit
number. Please preserve this convention, it simplifies viewing the
directory and understanding test order. Most of the time it's not
important but `00x` should be reserved for the times when it matters.


Analyzing test failures
=======================

The top priority for this scheme is to make it easy to diagnose
what went wrong. To that end, `podman_run` always logs all invoked
commands, their output and exit codes. In a normal run you will never
see this, but BATS will display it on failure. The goal here is to
give you everything you need to diagnose without having to rerun tests.

The `is` comparison function is designed to emit useful diagnostics,
in particular, the actual and expected strings. Please do not use
the horrible BATS standard of `[ x = y ]`; that's nearly useless
for tracking down failures.

If the above are not enough to help you track down a failure:


Debugging tests
---------------

Some functions have `dprint` statements. To see the output of these,
set `PODMAN_TEST_DEBUG="funcname"` where `funcname` is the name of
the function or perhaps just a substring.


Further Details
===============

TBD. For now, look in [helpers.bash](helpers.bash); each helper function
has (what are intended to be) helpful header comments. For even more
examples, see and/or run `helpers.t`; that's a regression test
and provides a thorough set of examples of how the helpers work.
