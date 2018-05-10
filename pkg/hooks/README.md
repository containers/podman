# OCI Hooks Configuration

For POSIX platforms, the [OCI runtime configuration][runtime-spec] supports [hooks][spec-hooks] for configuring custom actions related to the life cycle of the container.
The way you enable the hooks above is by editing the OCI runtime configuration before running the OCI runtime (e.g. [`runc`][runc]).
CRI-O and `podman create` create the OCI configuration for you, and this documentation allows developers to configure them to set their intended hooks.

One problem with hooks is that the runtime actually stalls execution of the container before running the hooks and stalls completion of the container, until all hooks complete.
This can cause some performance issues.
Also a lot of hooks just check if certain configuration is set and then exit early, without doing anything.
For example the [oci-systemd-hook][] only executes if the command is `init` or `systemd`, otherwise it just exits.
This means if we automatically enabled all hooks, every container would have to execute `oci-systemd-hook`, even if they don't run systemd inside of the container.
Performance would also suffer if we exectuted each hook at each stage ([pre-start][], [post-start][], and [post-stop][]).

## Notational Conventions

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "NOT RECOMMENDED", "MAY", and "OPTIONAL" are to be interpreted as described in [RFC 2119][rfc2119].

## JSON Definition

This package reads all [JSON][] files (ending with a `.json` extention) from a series of hook directories.
For both `crio` and `podman`, hooks are read from `/usr/share/containers/oci/hooks.d/*.json`.

For `crio`, hook JSON is also read from `/etc/containers/oci/hooks.d/*.json`.
If files of with the same name exist in both directories, the one in `/etc/containers/oci/hooks.d` takes precedence.

Hooks MUST be injected in the JSON filename case- and width-insensitive collation order.
Collation order depends on your locale, as set by [`LC_ALL`][LC_ALL], [`LC_COLLATE`][LC_COLLATE], or [`LANG`][LANG] (in order of decreasing precedence).
For example, in the [POSIX locale][LC_COLLATE-POSIX], a matching hook defined in `01-my-hook.json` would be injected before matching hooks defined in `02-another-hook.json` and `01-UPPERCASE.json`.

Each JSON file should contain an object with the following properties:

### 1.0.0 Hook Schema

* **`version`** (REQUIRED, string) Sets the hook-definition version.
    For this schema version, the value MUST be 1.0.0.
* **`hook`** (REQUIRED, object) The hook to inject, with the [hook-entry schema][spec-hooks] defined by the 1.0.1 OCI Runtime Specification.
* **`when`** (REQUIRED, object) Conditions under which the hook is injected.
    The following properties can be specified:

    * **`always`** (OPTIONAL, boolean) If set `true`, this condition matches.
    * **`annotations`** (OPTIONAL, object) If all `annotations` key/value pairs match a key/value pair from the [configured annotations][spec-annotations], this condition matches.
        Both keys and values MUST be [POSIX extended regular expressions][POSIX-ERE].
    * **`commands`** (OPTIONAL, array of strings) If the configured [`process.args[0]`][spec-process] matches an entry, this condition matches.
        Entries MUST be [POSIX extended regular expressions][POSIX-ERE].
    * **`hasBindMounts`** (OPTIONAL, boolean) If `hasBindMounts` is true and the caller requested host-to-container bind mounts (beyond those that CRI-O or libpod use by default), this condition matches.
* **`stages`** (REQUIRED, array of strings) Stages when the hook MUST be injected.
    Entries MUST be chosen from the 1.0.1 OCI Runtime Specification [hook stages][spec-hooks].

If *all* of the conditions set in `when` match, then the `hook` MUST be injected for the stages set in `stages`.

#### Example

The following configuration injects [`oci-systemd-hook`][oci-systemd-hook] in the [pre-start][] and [post-stop][] stages if [`process.args[0]`][spec-process] ends with `/init` or `/systemd`:

```console
$ cat /etc/containers/oci/hooks.d/oci-systemd-hook.json
{
  "version": "1.0.0",
  "hook": {
    "path": "/usr/libexec/oci/hooks.d/oci-systemd-hook"
  }
  "when": {
    "args": [".*/init$" , ".*/systemd$"],
  },
  "stages": ["prestart", "poststop"]
}
```

The following example injects [`oci-umount --debug`][oci-umount] in the [pre-start][] phase if the container is configured to bind-mount host directories into the container.

```console
$ cat /etc/containers/oci/hooks.d/oci-umount.json
{
  "version": "1.0.0",
  "hook": {
    "path": "/usr/libexec/oci/hooks.d/oci-umount",
    "args": ["oci-umount", "--debug"],
  }
  "when": {
    "hasBindMounts": true,
  },
  "stages": ["prestart"]
}
```

The following example injects [`nvidia-container-runtime-hook prestart`][nvidia-container-runtime-hook] with particular environment variables in the [pre-start][] phase if the container is configured with an `annotations` entry whose key matches `^com\.example\.department$` and whose value matches `.*fluid-dynamics.*`.

```console
$ cat /etc/containers/oci/hooks.d/nvidia.json
{
  "hook": {
    "path": "/usr/sbin/nvidia-container-runtime-hook",
    "args": ["nvidia-container-runtime-hook", "prestart"],
    "env": [
      "NVIDIA_REQUIRE_CUDA=cuda>=9.1",
      "NVIDIA_VISIBLE_DEVICES=GPU-fef8089b"
    ]
  },
  "when": {
    "annotations": {
      "^com\.example\.department$": ".*fluid-dynamics$"
    }
  },
  "stages": ["prestart"]
}
```

### 0.1.0 Hook Schema

Previous versions of CRI-O and libpod supported the 0.1.0 hook schema:

* **`hook`** (REQUIRED, string) Sets [`path`][spec-hooks] in the injected hook.
* **`arguments`** (OPTIONAL, array of strings) Additional arguments to pass to the hook.
    The injected hook's [`args`][spec-hooks] is `hook` with `arguments` appended.
* **`stages`** (REQUIRED, array of strings) Stages when the hook MUST be injected.
    `stage` is an allowed synonym for this property, but you MUST NOT set both `stages` and `stage`.
    Entries MUST be chosen from:
    * **`prestart`**, to inject [pre-start][].
    * **`poststart`**, to inject [post-start][].
    * **`poststop`**, to inject [post-stop][].
* **`cmds`** (OPTIONAL, array of strings) The hook MUST be injected if the configured [`process.args[0]`][spec-process] matches an entry.
    `cmd` is an allowed synonym for this property, but you MUST NOT set both `cmds` and `cmd`.
    Entries MUST be [POSIX extended regular expressions][POSIX-ERE].
* **`annotations`** (OPTIONAL, array of strings) The hook MUST be injected if an `annotations` entry matches a value from the [configured annotations][spec-annotations].
    `annotation` is an allowed synonym for this property, but you MUST NOT set both `annotations` and `annotation`.
    Entries MUST be [POSIX extended regular expressions][POSIX-ERE].
* **`hasbindmounts`** (OPTIONAL, boolean) The hook MUST be injected if `hasBindMounts` is true and the caller requested host-to-container bind mounts (beyond those that CRI-O or libpod use by default).

#### Example

The following configuration injects [`oci-systemd-hook`][oci-systemd-hook] in the [pre-start][] and [post-stop][] stages if [`process.args[0]`][spec-process] ends with `/init` or `/systemd`:

```console
$ cat /etc/containers/oci/hooks.d/oci-systemd-hook.json
{
  "cmds": [".*/init$" , ".*/systemd$"],
  "hook": "/usr/libexec/oci/hooks.d/oci-systemd-hook",
  "stages": ["prestart", "poststop"]
}
```

The following example injects [`oci-umount --debug`][oci-umount] in the [pre-start][] phase if the container is configured to bind-mount host directories into the container.

```console
$ cat /etc/containers/oci/hooks.d/oci-umount.json
{
  "hook": "/usr/libexec/oci/hooks.d/oci-umount",
  "arguments": ["--debug"],
  "hasbindmounts": true,
  "stages": ["prestart"]
}
```

The following example injects [`nvidia-container-runtime-hook prestart`][nvidia-container-runtime-hook] in the [pre-start][] phase if the container is configured with an `annotations` entry whose value matches `.*fluid-dynamics.*`.

```console
$ cat /etc/containers/oci/hooks.d/osystemd-hook.json
{
  "hook": "/usr/sbin/nvidia-container-runtime-hook",
  "arguments": ["prestart"],
  "annotations: [".*fluid-dynamics.*"],
  "stages": ["prestart"]
}
```

[JSON]: https://tools.ietf.org/html/rfc8259
[LANG]: http://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap08.html#tag_08_02
[LC_ALL]: http://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap08.html#tag_08_02
[LC_COLLATE]: http://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap07.html#tag_07_03_02
[LC_COLLATE-POSIX]: http://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap07.html#tag_07_03_02_06
[nvidia-container-runtime-hook]: https://github.com/NVIDIA/nvidia-container-runtime/tree/master/hook/nvidia-container-runtime-hook
[oci-systemd-hook]: https://github.com/projectatomic/oci-systemd-hook
[oci-umount]: https://github.com/projectatomic/oci-umount
[POSIX-ERE]: http://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap09.html#tag_09_04
[post-start]: https://github.com/opencontainers/runtime-spec/blob/v1.0.1/config.md#poststart
[post-stop]: https://github.com/opencontainers/runtime-spec/blob/v1.0.1/config.md#poststop
[pre-start]: https://github.com/opencontainers/runtime-spec/blob/v1.0.1/config.md#prestart
[rfc2119]: http://tools.ietf.org/html/rfc2119
[runc]: https://github.com/opencontainers/runc
[runtime-spec]: https://github.com/opencontainers/runtime-spec/blob/v1.0.1/spec.md
[spec-annotations]: https://github.com/opencontainers/runtime-spec/blob/v1.0.1/config.md#annotations
[spec-hooks]: https://github.com/opencontainers/runtime-spec/blob/v1.0.1/config.md#posix-platform-hooks
[spec-process]: https://github.com/opencontainers/runtime-spec/blob/v1.0.1/config.md#process
