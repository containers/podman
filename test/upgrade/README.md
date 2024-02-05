Background
==========

For years we've been needing a way to test podman upgrades; this
became much more critical on December 7, 2020, when Matt disclosed
a bug he had found over the weekend
([#8613](https://github.com/containers/podman/issues/8613))
in which reuse of a previously-defined field name would
result in fatal JSON decode failures if current-podman were
to try reading containers created with podman <= 1.8 (FIXME: confirm)

Upgrade testing is a daunting problem; but in the December 12
Cabal meeting Dan suggested using podman-in-podman. This PR
is the result of fleshing out that idea.

Overview
========

The BATS script in this directory fetches and runs an old-podman
container image from quay.io/podman, uses it to create and run
a number of containers, then uses new-podman to interact with
those containers.

As of 2024-02-05 the available old-podman versions are:

```console
$ bin/podman search --list-tags --limit=400 quay.io/podman/stable | awk '$2 ~ /^v/ { print $2}' | sort | column -c 75
v1.4.2  v1.9.1  v3.2.0  v3.4.0  v4.1.0  v4.3.1  v4.5.1  v4.8
v1.4.4  v2.0.2  v3.2.1  v3.4.1  v4.1.1  v4.4    v4.6    v4.8.0
v1.5.0  v2.0.6  v3.2.2  v3.4.2  v4.2    v4.4.1  v4.6.1  v4.8.1
v1.5.1  v2.1.1  v3.2.3  v3.4.4  v4.2.0  v4.4.2  v4.6.2  v4.8.2
v1.6    v2.2.1  v3.3.0  v3.4.7  v4.2.1  v4.4.4  v4.7    v4.8.3
v1.6.2  v3      v3.3.1  v4      v4.3    v4.5    v4.7.0  v4.9
v1.9.0  v3.1.2  v3.4    v4.1    v4.3.0  v4.5.0  v4.7.2  v4.9.0
```

Test invocation is:
```console
$ sudo env PODMAN=bin/podman PODMAN_UPGRADE_FROM=v4.1.0 PODMAN_UPGRADE_TEST_DEBUG= bats test/upgrade
```
(Path assumes you're cd'ed to top-level podman repo). `PODMAN_UPGRADE_FROM`
can be any of the versions above. `PODMAN_UPGRADE_TEST_DEBUG` is empty
here, but listed so you can set it `=1` and leave the podman_parent
container running. Interacting with this container is left as an
exercise for the reader.

The script will pull the given podman image, invoke it with a scratch
root directory, and have it do a small set of podman stuff (pull an
image, create/run some containers). This podman process stays running
because if it exits, it kills containers running inside the container.

We then invoke the current (host-installed) podman, using the same
scratch root directory, and perform operations on those images and
containers. Most of those operations are done in individual @tests.

The goal is to have this upgrade test run in CI, iterating over a
loop of known old versions. This list would need to be hand-maintained
and updated on new releases. There might also need to be extra
configuration defined, such as per-version commands (see below).

Findings
========

Well, first, `v1.6.2` won't work on default f32/f33: the image
does not include `crun`, so it can't work at all:

    ERRO[0000] oci runtime "runc" does not support CGroups V2: use system migrate to mitigate

I realize that it's kind of stupid not to test 1.6, since that's
precisely the test that would've caught #8613 early, but I just
don't think it's worth the hassle of setting up cgroupsv1 VMs.

For posterity, in an earlier incantation of this script I tried
booting f32 into cgroupsv1 and ran into the following warnings
when running new-podman on old-containers:
```
ERRO[0000] error joining network namespace for container 322b66d94640e31b2e6921565445cf0dade4ec13cabc16ee5f29292bdc038341: error retrieving network namespace at /var/run/netns/cni-577e2289-2c05-2e28-3c3d-002a5596e7da: failed to Statfs "/var/run/netns/cni-577e2289
```

Where To Go From Here
=====================

* Tests are still (2021-02-23) incomplete, with several failing outright.
  See FIXMEs in the code.

* Figuring out how/if to run rootless. I think this is possible, perhaps
  even necessary, but will be tricky to get right because of home-directory
  mounting.

* Figuring out how/if to run variations with different config files
  (e.g. running OLD-PODMAN that creates a user libpod.conf, tweaking
  that in the test, then running NEW-PODMAN upgrade tests)
