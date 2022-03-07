# Podman Packaging

This document is currently written with Fedora as a reference, intended for use
by packagers of other distros as well as package users.

## Fedora Users
Podman v4 is available as an official Fedora package on Fedora 36 and rawhide.
This version of Podman brings with it a new container stack called
Netavark which serves as a replacement for CNI plugins
(containernetworking-plugins on Fedora), as well as Aardvark-dns, the
authoritative DNS server for container records.

Both Netavark and Aardvark-dns are available as official Fedora packages on
Fedora 35 and newer versions and form the default network stack for new
installations of Podman 4.0.

On Fedora 36 and newer, fresh installations of Podman v4 will
automatically install Aardvark-dns along with Netavark.

To install Podman v4:

```console
$ sudo dnf install podman
```

To update Podman from an older version to v4:

```console
$ sudo dnf update podman
```

**NOTE:** Fedora 35 users will not be able to install Podman v4 using the default yum
repositories and are recommended to use the COPR repo below:

```console
$ sudo dnf copr enable rhcontainerbot/podman4

# install or update per your needs
$ sudo dnf install podman
```

After installation, if you would like to migrate all your containers to use
Netavark, you will need to set `network_backend = "netavark"` under
the `[network]` section in your containers.conf, typically located at:
`/usr/share/containers/containers.conf`

### Testing the latest development version`

If you would like to test the latest unreleased upstream code, try the
podman-next COPR

```console
$ sudo dnf copr enable rhcontainerbot/podman-next

$ sudo dnf install podman
```

**CAUTION:** The podman-next COPR provides the latest unreleased sources of Podman,
Netavark and Aardvark-dns as rpms which would override the versions provided by
the official packages.

## Distro Packagers

The Fedora packaging sources for Podman are available at the [Podman
dist-git](https://src.fedoraproject.org/rpms/podman).

The main `podman` package no longer explicitly depends on
containernetworking-plugins. The network stack dependencies are now handled in
the [containers-common](https://src.fedoraproject.org/rpms/containers-common)
package which allows for a single point of dependency maintenance for Podman
and Buildah.

- containers-common
```
Requires: container-network-stack
Recommends: netavark
```

- netavark
```
Provides: container-network-stack = 2
```

- containernetworking-plugins
```
Provides: container-network-stack = 1
```

This configuration ensures:
- New installations of Podman will always install netavark by default.
- The containernetworking-plugins package will not conflict with netavark and
users can install them together.

## Listing bundled dependencies
If you need to list the bundled dependencies in your packaging sources, you can
process the `go.mod` file in the upstream source.
For example, Fedora's packaging source uses:

```
$ awk '{print "Provides: bundled(golang("$1")) = "$2}' go.mod | sort | uniq | sed -e 's/-/_/g' -e '/bundled(golang())/d' -e '/bundled(golang(go\|module\|replace\|require))/d'
```
