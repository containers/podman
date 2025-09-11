% podman-quadlet(7)

# NAME

podman\-quadlet - High-level overview of Podman Quadlet integration with systemd

# DESCRIPTION

Podman Quadlet is a systemd unit generator that enables declarative container management by defining
Podman resources using specialized systemd unit files. It provides an interface between Podman and systemd,
allowing users to manage containers, pods, volumes, images, and more through native systemd service units.

Quadlet parses `.container`, `.pod`, `.volume`, `.network`, `.image`, `.build`, and `.kube` files located in
well-known directories and converts them into corresponding `.service` units for systemd. These units are then
used to create and manage container resources on boot or via explicit `systemctl` commands.

# PURPOSE

Quadlet's goal is to bridge the gap between container lifecycle management and system initialization. It lets
you define container infrastructure declaratively, leveraging systemd's features such as dependency ordering,
restart policies, boot-time activation, logging, and drop-in configuration.

It eliminates the need to write complex `ExecStart=` lines or manage custom systemd services manually.

# BENEFITS

- Declarative and human-readable unit definitions
- Seamless integration with native systemd features
- Consistent naming and dependency management
- Support for advanced Podman features (pods, kube, image builds)
- Rootless and rootful compatibility
- Drop-in override support for all unit types

# INTEGRATION WITH SYSTEMD

At boot time, the `podman-system-generator` reads Quadlet unit files from predefined directories and translates
them into `.service` files under `/run/systemd/generator/`. These generated services are treated like native
systemd units and can be enabled, started, and inspected using standard tools such as:

```bash
systemctl enable my-container.service
systemctl start my-container.service
journalctl -u my-container.service
```

Quadlet integrates cleanly with both rootless and rootful Podman environments, depending on the search paths used.

# FILE TYPES

Quadlet supports the following file types:

- **`.container`** — Defines and manages a single container. See [podman-container.unit(5)](podman-container.unit.5.md).
- **`.pod`** — Creates a Podman pod that containers can join. See [podman-pod.unit(5)](podman-pod.unit.5.md).
- **`.volume`** — Ensures a named Podman volume exists. See [podman-volume.unit(5)](podman-volume.unit.5.md).
- **`.network`** — Creates a Podman network for containers and pods. See [podman-network.unit(5)](podman-network.unit.5.md).
- **`.image`** — Pulls and caches a container image. See [podman-image.unit(5)](podman-image.unit.5.md).
- **`.build`** — Builds a container image from a Containerfile. See [podman-build.unit(5)](podman-build.unit.5.md).
- **`.kube`** — Deploys containers from Kubernetes YAML using [podman-kube.unit(5)](podman-kube.unit.5.md).

Each file is mapped to a corresponding `.service` unit with a predictable naming pattern, typically appending
`-<type>.service` to the unit base name.

# FILE PATHS

Quadlet files should be stored in specific directories depending on rootless or rootful execution.

## Rootful

- `/run/containers/systemd/`
- `/etc/containers/systemd/`
- `/usr/share/containers/systemd/`

## Rootless

- `$XDG_RUNTIME_DIR/containers/systemd/`
- `$XDG_CONFIG_HOME/containers/systemd/` or `~/.config/containers/systemd/`
- `/etc/containers/systemd/users/$(UID)`
- `/etc/containers/systemd/users/`

## QUADLET SECTION [Quadlet]
Some quadlet specific configuration is shared between different unit types. Those settings
can be configured in the `[Quadlet]` section.

Valid options for `[Quadlet]` are listed below:

| **[Quadlet] options**      | **Description**                                   |
|----------------------------|---------------------------------------------------|
| DefaultDependencies=false  | Disable implicit network dependencies to the unit |

### `DefaultDependencies=`

Add Quadlet's default network dependencies to the unit (default is `true`).

When set to false, Quadlet will **not** add a dependency (After=, Wants=) to
`network-online.target`/`podman-user-wait-network-online.service` to the generated unit.

Note, this option is set in the `[Quadlet]` section. The _systemd_ `[Unit]` section
has an option with the same name but a different meaning.

# SEE ALSO

[podman-quadlet(7)](https://docs.podman.io/en/latest/markdown/podman-quadlet.7.html),
[podman-container.unit(5)](podman-container.unit.5.md),
[podman-pod.unit(5)](podman-pod.unit.5.md),
[podman-volume.unit(5)](podman-volume.unit.5.md),
[podman-network.unit(5)](podman-network.unit.5.md),
[podman-image.unit(5)](podman-image.unit.5.md),
[podman-build.unit(5)](podman-build.unit.5.md),
[podman-kube.unit(5)](podman-kube.unit.5.md),
[systemd.unit(5)](https://www.freedesktop.org/software/systemd/man/systemd.unit.html)

# AUTHORS

Podman Team <https://podman.io>
