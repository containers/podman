# oci-runtime-tool [![Build Status](https://travis-ci.org/opencontainers/runtime-tools.svg?branch=master)](https://travis-ci.org/opencontainers/runtime-tools) [![Go Report Card](https://goreportcard.com/badge/github.com/opencontainers/runtime-tools)](https://goreportcard.com/report/github.com/opencontainers/runtime-tools)

oci-runtime-tool is a collection of tools for working with the [OCI runtime specification][runtime-spec].
To build from source code, runtime-tools requires Go 1.7.x or above.

## Generating an OCI runtime spec configuration files

[`oci-runtime-tool generate`][generate.1] generates [configuration JSON][config.json] for an [OCI bundle][bundle].
[OCI-compatible runtimes][runtime-spec] like [runC][] expect to read the configuration from `config.json`.

```sh
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

```sh
$ oci-runtime-tool generate
$ oci-runtime-tool validate
INFO[0000] Bundle validation succeeded.
```

## Testing OCI runtimes

```sh
$ sudo make RUNTIME=runc localvalidation
RUNTIME=runc go test -tags ""  -v github.com/opencontainers/runtime-tools/validation
=== RUN   TestValidateBasic
TAP version 13
ok 1 - root filesystem
ok 2 - hostname
ok 3 - mounts
ok 4 - capabilities
ok 5 - default symlinks
ok 6 - default devices
ok 7 - linux devices
ok 8 - linux process
ok 9 - masked paths
ok 10 - oom score adj
ok 11 - read only paths
ok 12 - rlimits
ok 13 - sysctls
ok 14 - uid mappings
ok 15 - gid mappings
1..15
--- PASS: TestValidateBasic (0.08s)
=== RUN   TestValidateSysctls
TAP version 13
ok 1 - root filesystem
ok 2 - hostname
ok 3 - mounts
ok 4 - capabilities
ok 5 - default symlinks
ok 6 - default devices
ok 7 - linux devices
ok 8 - linux process
ok 9 - masked paths
ok 10 - oom score adj
ok 11 - read only paths
ok 12 - rlimits
ok 13 - sysctls
ok 14 - uid mappings
ok 15 - gid mappings
1..15
--- PASS: TestValidateSysctls (0.20s)
PASS
ok      github.com/opencontainers/runtime-tools/validation      0.281s
```

[bundle]: https://github.com/opencontainers/runtime-spec/blob/master/bundle.md
[config.json]: https://github.com/opencontainers/runtime-spec/blob/master/config.md
[runC]: https://github.com/opencontainers/runc
[runtime-spec]: https://github.com/opencontainers/runtime-spec

[generate.1]: man/oci-runtime-tool-generate.1.md
[validate.1]: man/oci-runtime-tool-validate.1.md
