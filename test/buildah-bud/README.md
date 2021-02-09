buildah-bud tests under podman
==============================

This directory contains tools for running 'buildah bud' tests
under podman. The key concept of the workflow is:

* Pull buildah @ version specified in go.mod
* Apply a small set of patches to buildah's tests directory, such that
  * BATS will use 'podman build' instead of 'buildah bud'; and
  * some not-applicable-under-podman tests are skipped

It's a teeny bit more complicated than that, but that's really most of
what you need to know for most purposes. The tests run in podman CI,
and for the most part are expected to just pass.

Troubleshooting
---------------

If you're reading this, it's probably because something went wrong.
At the time of this writing (March 2021, initial commit) it is
impossible to foresee what typical failures will look like, but
my prediction is that they will fit one of two categories:

* Failure when vendoring new buildah (e.g., by dependabot)
* Other failure

Let's examine those in reverse order:

Failure when not vendoring
--------------------------

Aside from flakes, my only guess here is that you broke 'podman build'.
If this is the case, it is very likely that you are aware of what you
did; and if this is the case, your change likely falls into one of
these two categories:

* "OOPS! I didn't mean to break that". Solution: fix it!
* "Uh, yeah, this is deliberate, and we choose to be incompatible with buildah". In this case, you'll need to skip or edit the failing test(s); see below.

If neither of those is the case, then I'm sorry, you're on your own.
When you figure it out, please remember to update these instructions.


Failure when vendoring new buildah
----------------------------------

This is what I predict will be the usual case; and I predict that
failures will fall into one of two bins:

* failure to apply the patch
* failure because there are new buildah tests for functionality not in podman

In either case, the process for solving is the same:

* Start with a checked-out podman tree with the failing PR applied
* run `./test/buildah-bud/run-buildah-bud-tests`

Presumably, something will fail here. Whatever the failure, your next step is:

* `cd test-buildah-v<TAB>` (this is a new directory created by the script)

If the failure was in `git am`, solve it (left as exercise for the reader).

If the failure was in tests run, solve it (either by adding `skip`s to
failing tests in bud.bats, or less preferably, by making other tweaks
to the test code).

You now have modified files. THOSE SHOULD ONLY BE test/bud.bats or
test/helpers.bash! If you changed any other file, that is a sign that
something is very wrong!

Commit your changes: `git commit --all --amend`

Push those changes to the podman repo: `./make-new-buildah-diffs`

cd back up to the podman repo

As necessary, rerun `run-buildah-bud-tests`. You can use `--no-checkout`
to run tests immediately, without rerunning the git checkout.

If you're happy with the diffs, `git add` the modified `.diff` file
and submit it as part of your PR.
