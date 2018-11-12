![PODMAN logo](logo/podman-logo-source.svg)
# Contributing to Libpod

We'd love to have you join the community! Below summarizes the processes
that we follow.

## Topics

* [Reporting Issues](#reporting-issues)
* [Submitting Pull Requests](#submitting-pull-requests)
* [Communications](#communications)

## Reporting Issues

Before reporting an issue, check our backlog of
[open issues](https://github.com/containers/libpod/issues)
to see if someone else has already reported it. If so, feel free to add
your scenario, or additional information, to the discussion. Or simply
"subscribe" to it to be notified when it is updated.

If you find a new issue with the project we'd love to hear about it! The most
important aspect of a bug report is that it includes enough information for
us to reproduce it. So, please include as much detail as possible and try
to remove the extra stuff that doesn't really relate to the issue itself.
The easier it is for us to reproduce it, the faster it'll be fixed!

Please don't include any private/sensitive information in your issue!

## Submitting Pull Requests

No Pull Request (PR) is too small! Typos, additional comments in the code,
new test cases, bug fixes, new features, more documentation, ... it's all
welcome!

While bug fixes can first be identified via an "issue", that is not required.
It's ok to just open up a PR with the fix, but make sure you include the same
information you would have included in an issue - like how to reproduce it.

PRs for new features should include some background on what use cases the
new code is trying to address. When possible and when it makes sense, try to break-up
larger PRs into smaller ones - it's easier to review smaller
code changes. But only if those smaller ones make sense as stand-alone PRs.

Regardless of the type of PR, all PRs should include:
* well documented code changes
* additional testcases. Ideally, they should fail w/o your code change applied
* documentation changes

Squash your commits into logical pieces of work that might want to be reviewed
separate from the rest of the PRs. But, squashing down to just one commit is ok
too since in the end the entire PR will be reviewed anyway. When in doubt,
squash.

PRs that fix issues should include a reference like `Closes #XXXX` in the
commit message so that GitHub will automatically close the referenced issue
when the PR is merged.

PRs will be approved by an [approver][owners] listed in [`OWNERS`](OWNERS).

### Describe your Changes in Commit Messages

Describe your problem. Whether your patch is a one-line bug fix or 5000 lines
of a new feature, there must be an underlying problem that motivated you to do
this work. Convince the reviewer that there is a problem worth fixing and that
it makes sense for them to read past the first paragraph.

Describe user-visible impact. Straight up crashes and lockups are pretty
convincing, but not all bugs are that blatant. Even if the problem was spotted
during code review, describe the impact you think it can have on users. Keep in
mind that the majority of users run packages provided by distributions, so
include anything that could help route your change downstream.

Quantify optimizations and trade-offs. If you claim improvements in
performance, memory consumption, stack footprint, or binary size, include
numbers that back them up. But also describe non-obvious costs. Optimizations
usually aren’t free but trade-offs between CPU, memory, and readability; or,
when it comes to heuristics, between different workloads. Describe the expected
downsides of your optimization so that the reviewer can weigh costs against
benefits.

Once the problem is established, describe what you are actually doing about it
in technical detail. It’s important to describe the change in plain English for
the reviewer to verify that the code is behaving as you intend it to.

Solve only one problem per patch. If your description starts to get long,
that’s a sign that you probably need to split up your patch.

If the patch fixes a logged bug entry, refer to that bug entry by number and
URL. If the patch follows from a mailing list discussion, give a URL to the
mailing list archive.

However, try to make your explanation understandable without external
resources. In addition to giving a URL to a mailing list archive or bug,
summarize the relevant points of the discussion that led to the patch as
submitted.

If you want to refer to a specific commit, don’t just refer to the SHA-1 ID of
the commit. Please also include the oneline summary of the commit, to make it
easier for reviewers to know what it is about. Example:

```
Commit f641c2d9384e ("fix bug in rm -fa parallel deletes") [...]
```

You should also be sure to use at least the first twelve characters of the
SHA-1 ID. The libpod repository holds a lot of objects, making collisions with
shorter IDs a real possibility. Bear in mind that, even if there is no
collision with your six-character ID now, that condition may change five years
from now.

If your patch fixes a bug in a specific commit, e.g. you found an issue using
git bisect, please use the ‘Fixes:’ tag with the first 12 characters of the
SHA-1 ID, and the one line summary. For example:

```
Fixes: f641c2d9384e ("fix bug in rm -fa parallel deletes")
```

The following git config settings can be used to add a pretty format for
outputting the above style in the git log or git show commands:

```
[core]
        abbrev = 12
[pretty]
        fixes = Fixes: %h (\"%s\")
```

### Sign your PRs

The sign-off is a line at the end of the explanation for the patch. Your
signature certifies that you wrote the patch or otherwise have the right to pass
it on as an open-source patch. The rules are simple: if you can certify
the below (from [developercertificate.org](http://developercertificate.org/)):

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
660 York Street, Suite 102,
San Francisco, CA 94110 USA

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.

Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

Then you just add a line to every git commit message:

    Signed-off-by: Joe Smith <joe.smith@email.com>

Use your real name (sorry, no pseudonyms or anonymous contributions.)

If you set your `user.name` and `user.email` git configs, you can sign your
commit automatically with `git commit -s`.

### Go Format and lint

All code changes must pass ``make validate`` and ``make lint``, as
executed in a standard container.  The container image for this
purpose is provided at: ``quay.io/libpod/gate:latest``.  However,
for changes to the image itself, it may also be built locally
from the repository root, with the command:

```
sudo podman build -t quay.io/libpod/gate:latest -f contrib/gate/Dockerfile .
```

The container executes 'make' by default, on a copy of the repository.
This avoids changing or leaving build artifacts in your working directory.
Execution does not require any special permissions from the host. However,
the repository root must be bind-mounted into the container at
'/usr/src/libpod'. For example, running `make lint` is done (from
the repository root) with the command:

``sudo podman run -it --rm -v $PWD:/usr/src/libpod:z quay.io/libpod/gate:latest lint``

### Integration Tests

Our primary means of performing integration testing for libpod is with the
[Ginkgo](https://github.com/onsi/ginkgo) BDD testing framework. This allows
us to use native Golang to perform our tests and there is a strong affiliation
between Ginkgo and the Go test framework.  Adequate test cases are expected to
be provided with PRs.

For details on how to run the tests for Podman in your test environment, see the
Integration Tests [README.md](test/README.md).

## Communications

For general questions and discussion, please use the
IRC `#podman` channel on `irc.freenode.net`.

For discussions around issues/bugs and features, you can use the GitHub
[issues](https://github.com/containers/libpod/issues)
and
[PRs](https://github.com/containers/libpod/pulls)
tracking system.

[owners]: https://github.com/kubernetes/community/blob/master/contributors/guide/owners.md#owners
