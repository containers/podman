.. include:: includes.rst

Configuration Files
========

:doc:`containers.conf <markdown/containers.conf.5>` The containers configuration file specifies all of the available configuration command-line options/flags for container engine tools like Podman & Buildah, but in a TOML format that can be easily modified and versioned.

:doc:`policy.json <markdown/containers-policy.json.5>` Signature verification policy files are used to specify policy,  e.g., trusted keys, applicable when deciding whether to accept an image or individual signatures of that image as valid.

:doc:`podman-systemd.unit <markdown/podman-systemd.unit.5>` systemd units using Podman Quadlet

:doc:`quadlet <markdown/podman-sysemd.unit.5>` systemd units using Podman Quadlet

:doc:`mount.conf <markdown/containers-mount.conf.5>` Configuration file for default mounts in containers.

:doc:`registries.conf <markdown/containers-registries.conf.5>` Configuration file is a system-wide configuration file for container image registries. The file format is TOML.

:doc:`storage.conf <markdown/containers-storage.conf.5>` Configuration file for all tools that use the containers/storage library.
