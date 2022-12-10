![PODMAN logo](https://raw.githubusercontent.com/containers/common/main/logos/podman-logo-full-vert.png)

# Downloads

## Latest signed/official

[The latest Podman release version is always available on the GitHub releases
page](https://github.com/containers/podman/releases/latest).  These are official,
signed, sealed, and blessed artifacts intended for general use.  Though for
super-serious production use, please utilize the pre-packaged podman provided
by your OS/Distro vendor.

## CI Artifacts

If you're looking for something even more bleeding-edge, esp. for testing
purposes and/or in other CI systems.  There are several permalinks available
depending on how much you want to download.  Everything inside has at least
gone through and passed CI testing.  However, **they are all unsigned**, and
frequently changing.  Perfectly fine for non-production testing but please
don't take them beyond that.

* [Giant artifacts
  archive](https://api.cirrus-ci.com/v1/artifact/github/containers/podman/Artifacts/binary.zip)
  containing every binary produced in CI from the most recent successful run.
  *Warning*: This file is pretty large, expect a 700+MB download.  However,
  it's guaranteed to contain everything, where as the items below can change
  or become unavailable due to somebody forgetting to update this doc.

<!--

WARNING:  The items linked below all come from scripts in the `artifacts_task`
map of `.cirrus.yml`.  When adding or updating any item below, please ensure it
matches corresponding changes in the artifacts task.

-->

* Raw dynamically linked ELF (x86_64) binaries for [podman](https://api.cirrus-ci.com/v1/artifact/github/containers/podman/Artifacts/binary/podman)
  , [podman-remote](https://api.cirrus-ci.com/v1/artifact/github/containers/podman/Artifacts/binary/podman-remote)
  , [quadlet](https://api.cirrus-ci.com/v1/artifact/github/containers/podman/Artifacts/binary/quadlet)
  , and
  [rootlessport](https://api.cirrus-ci.com/v1/artifact/github/containers/podman/Artifacts/binary/rootlessport) -
  Built on the latest supported Fedora release.
* MacOS,
  [both x86_64](https://api.cirrus-ci.com/v1/artifact/github/containers/podman/Artifacts/binary/podman-installer-macos-amd64.pkg)
  and
  [aarch64 (ARM)](https://api.cirrus-ci.com/v1/artifact/github/containers/podman/Artifacts/binary/podman-installer-macos-aarch64.pkg)
  installation packages.  Again, these are **not** signed, so expect warnings. There's
  also binary release *ZIP-files* for
  [darwin_amd64](https://api.cirrus-ci.com/v1/artifact/github/containers/podman/Artifacts/binary/podman-remote-release-darwin_amd64.zip)
  and
  [darwin_arm64](https://api.cirrus-ci.com/v1/artifact/github/containers/podman/Artifacts/binary/podman-remote-release-darwin_arm64.zip).
  if you try to install them.
* Windows [podman-remote](https://api.cirrus-ci.com/v1/artifact/github/containers/podman/Artifacts/binary/podman.msi)
  for x86_64 only.
* Other podman-remote release builds (includes configuration files & documentation):
  * [podman-release-386.tar.gz](https://api.cirrus-ci.com/v1/artifact/github/containers/podman/Artifacts/binary/podman-release-386.tar.gz)
  * [podman-release-arm.tar.gz](https://api.cirrus-ci.com/v1/artifact/github/containers/podman/Artifacts/binary/podman-release-arm.tar.gz)
  * [podman-release-arm64.tar.gz](https://api.cirrus-ci.com/v1/artifact/github/containers/podman/Artifacts/binary/podman-release-arm64.tar.gz)
  * [podman-release-mips.tar.gz](https://api.cirrus-ci.com/v1/artifact/github/containers/podman/Artifacts/binary/podman-release-mips.tar.gz)
  * [podman-release-mips64.tar.gz](https://api.cirrus-ci.com/v1/artifact/github/containers/podman/Artifacts/binary/podman-release-mips64.tar.gz)
  * [podman-release-mips64le.tar.gz](https://api.cirrus-ci.com/v1/artifact/github/containers/podman/Artifacts/binary/podman-release-mips64le.tar.gz)
  * [podman-release-mipsle.tar.gz](https://api.cirrus-ci.com/v1/artifact/github/containers/podman/Artifacts/binary/podman-release-mipsle.tar.gz)
  * [podman-release-ppc64le.tar.gz](https://api.cirrus-ci.com/v1/artifact/github/containers/podman/Artifacts/binary/podman-release-ppc64le.tar.gz)
  * [podman-release-s390x.tar.gz](https://api.cirrus-ci.com/v1/artifact/github/containers/podman/Artifacts/binary/podman-release-s390x.tar.gz)
