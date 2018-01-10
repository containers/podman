# oci-runtime-tool [![Build Status](https://travis-ci.org/opencontainers/runtime-tools.svg?branch=master)](https://travis-ci.org/opencontainers/runtime-tools) [![Go Report Card](https://goreportcard.com/badge/github.com/opencontainers/runtime-tools)](https://goreportcard.com/report/github.com/opencontainers/runtime-tools)

oci-runtime-tool is a collection of tools for working with the [OCI runtime specification][runtime-spec].
To build from source code, runtime-tools requires Go 1.7.x or above.

## Generating an OCI runtime spec configuration files

[`oci-runtime-tool generate`][generate.1] generates [configuration JSON][config.json] for an [OCI bundle][bundle].
[OCI-compatible runtimes][runtime-spec] like [runC][] expect to read the configuration from `config.json`.

```console
$ oci-runtime-tool generate --output config.json
$ cat config.json
{
        "ociVersion": "0.5.0",
        …
}
```

## Validating an OCI bundle

[`oci-runtime-tool validate`][validate.1] validates an OCI bundle.
The error message will be printed if the OCI bundle failed the validation procedure.

```console
$ oci-runtime-tool generate
$ oci-runtime-tool validate
INFO[0000] Bundle validation succeeded.
```

## Testing OCI runtimes

The runtime validation suite uses [node-tap][], which is packaged for some distributions (for example, it is in [Debian's `node-tap` package][debian-node-tap]).
If your distribution does not package node-tap, you can install [npm][] (for example, from [Gentoo's `nodejs` package][gentoo-nodejs]) and use it:

```console
$ npm install tap
```

```console
$ make runtimetest validation-executables
RUNTIME=runc tap validation/linux_rootfs_propagation_shared.t validation/create.t validation/default.t validation/linux_readonly_paths.t validation/linux_masked_paths.t validation/mounts.t validation/process.t validation/root_readonly_false.t validation/linux_sysctl.t validation/linux_devices.t validation/linux_gid_mappings.t validation/process_oom_score_adj.t validation/process_capabilities.t validation/process_rlimits.t validation/root_readonly_true.t validation/linux_rootfs_propagation_unbindable.t validation/hostname.t validation/linux_uid_mappings.t
validation/linux_rootfs_propagation_shared.t ........ 18/19
  not ok rootfs propagation

validation/create.t ................................... 4/4
validation/default.t ................................ 19/19
validation/linux_readonly_paths.t ................... 19/19
validation/linux_masked_paths.t ..................... 18/19
  not ok masked paths

validation/mounts.t ................................... 0/1
  Skipped: 1
     TODO: mounts generation options have not been implemented

validation/process.t ................................ 19/19
validation/root_readonly_false.t .................... 19/19
validation/linux_sysctl.t ........................... 19/19
validation/linux_devices.t .......................... 19/19
validation/linux_gid_mappings.t ..................... 18/19
  not ok gid mappings

validation/process_oom_score_adj.t .................. 19/19
validation/process_capabilities.t ................... 19/19
validation/process_rlimits.t ........................ 19/19
validation/root_readonly_true.t ...................failed to create the container
rootfsPropagation=unbindable is not supported
exit status 1
validation/root_readonly_true.t ..................... 19/19
validation/linux_rootfs_propagation_unbindable.t ...... 0/1
  not ok validation/linux_rootfs_propagation_unbindable.t
    timeout: 30000
    file: validation/linux_rootfs_propagation_unbindable.t
    command: validation/linux_rootfs_propagation_unbindable.t
    args: []
    stdio:
      - 0
      - pipe
      - 2
    cwd: /…/go/src/github.com/opencontainers/runtime-tools
    exitCode: 1

validation/hostname.t ...................failed to create the container
User namespace mappings specified, but USER namespace isn't enabled in the config
exit status 1
validation/hostname.t ............................... 19/19
validation/linux_uid_mappings.t ....................... 0/1
  not ok validation/linux_uid_mappings.t
    timeout: 30000
    file: validation/linux_uid_mappings.t
    command: validation/linux_uid_mappings.t
    args: []
    stdio:
      - 0
      - pipe
      - 2
    cwd: /…/go/src/github.com/opencontainers/runtime-tools
    exitCode: 1

total ............................................. 267/273


  267 passing (31s)
  1 pending
  5 failing

make: *** [Makefile:43: localvalidation] Error 1
```

You can also run an individual test executable directly:

```console
$ RUNTIME=runc validation/default.t
TAP version 13
ok 1 - root filesystem
ok 2 - hostname
ok 3 - process
ok 4 - mounts
ok 5 - user
ok 6 - rlimits
ok 7 - capabilities
ok 8 - default symlinks
ok 9 - default file system
ok 10 - default devices
ok 11 - linux devices
ok 12 - linux process
ok 13 - masked paths
ok 14 - oom score adj
ok 15 - read only paths
ok 16 - rootfs propagation
ok 17 - sysctls
ok 18 - uid mappings
ok 19 - gid mappings
1..19
```

If you cannot install node-tap, you can probably run the test suite with another [TAP consumer][tap-consumers].
For example, with [`prove`][prove]:

```console
$ sudo make TAP='prove -Q -j9' RUNTIME=runc localvalidation
RUNTIME=runc prove -Q -j9 validation/linux_rootfs_propagation_shared.t validation/create.t validation/default.t validation/linux_readonly_paths.t validation/linux_masked_paths.t validation/mounts.t validation/process.t validation/root_readonly_false.t validation/linux_sysctl.t validation/linux_devices.t validation/linux_gid_mappings.t validation/process_oom_score_adj.t validation/process_capabilities.t validation/process_rlimits.t validation/root_readonly_true.t validation/linux_rootfs_propagation_unbindable.t validation/hostname.t validation/linux_uid_mappings.t
failed to create the container
rootfsPropagation=unbindable is not supported
exit status 1
failed to create the container
User namespace mappings specified, but USER namespace isn't enabled in the config
exit status 1

Test Summary Report
-------------------
validation/linux_rootfs_propagation_shared.t    (Wstat: 0 Tests: 19 Failed: 1)
  Failed test:  16
validation/linux_masked_paths.t                 (Wstat: 0 Tests: 19 Failed: 1)
  Failed test:  13
validation/linux_rootfs_propagation_unbindable.t (Wstat: 256 Tests: 0 Failed: 0)
  Non-zero exit status: 1
  Parse errors: No plan found in TAP output
validation/linux_uid_mappings.t                 (Wstat: 256 Tests: 0 Failed: 0)
  Non-zero exit status: 1
  Parse errors: No plan found in TAP output
validation/linux_gid_mappings.t                 (Wstat: 0 Tests: 19 Failed: 1)
  Failed test:  19
Files=18, Tests=271,  6 wallclock secs ( 0.06 usr  0.01 sys +  0.59 cusr  0.24 csys =  0.90 CPU)
Result: FAIL
make: *** [Makefile:43: localvalidation] Error 1
```

[bundle]: https://github.com/opencontainers/runtime-spec/blob/master/bundle.md
[config.json]: https://github.com/opencontainers/runtime-spec/blob/master/config.md
[debian-node-tap]: https://packages.debian.org/stretch/node-tap
[debian-nodejs]: https://packages.debian.org/stretch/nodejs
[gentoo-nodejs]: https://packages.gentoo.org/packages/net-libs/nodejs
[node-tap]: http://www.node-tap.org/
[npm]: https://www.npmjs.com/
[prove]: http://search.cpan.org/~leont/Test-Harness-3.39/bin/prove
[runC]: https://github.com/opencontainers/runc
[runtime-spec]: https://github.com/opencontainers/runtime-spec
[tap-consumers]: https://testanything.org/consumers.html

[generate.1]: man/oci-runtime-tool-generate.1.md
[validate.1]: man/oci-runtime-tool-validate.1.md
