# End-of-Life Policy on Issues

_jetsam (n): the part of a ship, its equipment, or its cargo that is cast overboard to lighten the load in time of distress_

Finite resources demand some level of pruning. This document describes
the basic principles used by the Containers team to identify and close
stale issues.

------

## Definitions

[**stale**](https://github.com/containers/podman/issues?q=is%3Aopen+is%3Aissue+sort%3Acreated-asc+label%3Astale-issue): open, but no activity in the last thirty days.

**crickets**: closed due to lack of response from reporting party.

[**jetsam**](https://github.com/containers/podman/issues?q=is%3Aissue+label%3Ajetsam+is%3Aclosed): closed without being implemented. A deliberate decision made in recognition of human limitations.

------

## Process

In order of judgment, from least to most.

#### &rarr; stale

Issues are marked with the label *stale-issue* by a [github action](https://github.com/containers/podman/blob/main/.github/workflows/stale.yml) that runs daily at 00:00 UT. This also triggers an email alert to subscribers on that issue.

Judgment: typically a team member will skim the issue, then decide whether to:

* remove the label; or
* close the issue (see below); or
* do nothing.

This is informal: there is no guarantee that anyone will actually do this.

#### &rarr; crickets

Typically done by a team member after receiving a *stale-issue* email.

Judgment:

* there is not enough information to act on the issue; and
* someone on the team has asked the reporter for more details (like NEEDINFO); and
* the reporter has not responded.

There is no actual *crickets* label. There is no automated way to
find issues that have been closed for this reason.

#### &rarr; jetsam

Last-resort closing of an issue that will not be worked on.

Factors:

* issue has remained open for over sixty days; and
* reporter is responsive, and still wishes to have the issue addressed (as does the team).

Judgment:

* the issue is too difficult or complicated or hard to track down.
* decision should be made by two or more team members, with discussion in the issue/PR itself.

When such an issue is closed, team member should apply the *jetsam* label.

------

## Grayer Areas

`stalebot` isn't perfect. It often misses issues, and we end up with
some that have been open a long time and do not have the `stale-issue` label.

These are hard to find manually. There is no defined process for identifying
or acting on them. If and when someone finds these, they should be handled
through the process defined above.
