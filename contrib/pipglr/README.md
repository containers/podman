## Overview

This container image is built daily from this `Containerfile`, and
made available as:

* quay.io/FIXME/FIXME

It's purpose is to provide an easy method to execute a GitLab runner,
to service CI/CD jobs for groups and/or repositories on
[gitlab.com](https://gitlab.com).  It comes pre-configured to utilize
the gitlab-runner app to execute with rootless podman containers,
nested inside a rootless podman container.

This is intended to provide multiple additional layers of security
for the host, when running potentially arbitrary CI/CD code.  Though,
the ultimate responsibility still rests with the end-user to review
the setup and configuration relative to their own situation/environment.

### Quickstart

Several labels are set on the built image or manifest list to support
easy registration and execution of a runner container.  They require
defining several environment variables for use.

#### Runner registration

Each time the registration command is run, a new runner is added into
the configuration.  If your intent is to simply update or modify the
configuration, please edit the config.toml file within the
`gitlab-runner-config` volume.

Note: These commands assume you have both `podman` and `jq` available.

Assuming you want to register four runners:
```bash
$ echo '<registration token>' | podman secret create REGISTRATION_TOKEN -
$ podman container runlabel register quay.io/FIXME/FIXME
$ podman container runlabel register quay.io/FIXME/FIXME
$ podman container runlabel register quay.io/FIXME/FIXME
$ podman container runlabel register quay.io/FIXME/FIXME
```

For older podman releases, without `container runlabel` support, you
may simulate it with:
```bash
$ echo '<registration token>' | podman secret create REGISTRATION_TOKEN -
$ export IMAGE=<image FQIN:TAG>
$ eval $(podman inspect --format=json $IMAGE | jq -r .[].Labels.register)
$ eval $(podman inspect --format=json $IMAGE | jq -r .[].Labels.register)
$ eval $(podman inspect --format=json $IMAGE | jq -r .[].Labels.register)
$ eval $(podman inspect --format=json $IMAGE | jq -r .[].Labels.register)
```

#### Runner Startup

With one or more runners registered and configured, and `$IMAGE` set,
the GitLab runner container may be launched with the following commands.

Note: The first time this is run, startup will take an extended amount
of time as the runner downloads and runs several (inner) support containers.

Debugging: You may `export PODMAN_RUNNER_DEBUG=debug` to enable inner-podman
debugging (or any other supported log level) to stdout.

```bash
$ eval $(podman inspect --format=json $IMAGE | jq -r .[].Labels.run)
```

## Building locally

This image may be built locally simply with:

`podman build -t runner .`

This will utilize the latest stable version of podman and the latest
stable version of the gitlab runner.

### Multi-arch

Assuming the host supports foreign-architecture emulation.  The
`Containerfile` may be used to produce a multi-arch manifest-list.
For example:

`podman build --jobs 4 --platform linux/amd64,linux/arm64 --manifest runner .`

### Build-args

Several build arguments are available to control the output image:

* `FLAVOR` - Choose from 'stable', 'testing', or 'upstream'.  These
  select the podman base-image to utilize - which may affect the
  podman version, features, and stability.  For more information
  see [the podmanimage README](https://github.com/containers/podman/blob/main/contrib/podmanimage/README.md).
* `BASE_TAG` - When `FLAVOR="stable"`, allows granular choice over the
  exact podman version.  Possible values include, `latest`, `vX`, `vX.Y`,
  and `vX.Y.Z` (where, `X`, `Y`, and `Z` represent the podman semantic
  version numbers).  It's also possible to specify an image SHA.
* `EXCLUDE_PACKAGES` - A space-separated list of RPM packages to prevent
  their existance in the final image.  This is intended as a security measure
  to limit the attack-surface should a gitlab-runner process escape it's
  inner-container.
* `RUNNER_VERSION` - Allows specifying an exact gitlab runner version.
  By default the `latest` is used, assuming the user is building a tagged
  image anyway.  Valid versions may be found on the [runner
  release page](https://gitlab.com/gitlab-org/gitlab-runner/-/releases).
* `TARGETARCH` - Supports inclusion of non-x86_64 gitlab runners.  This
   value is assumed to match the image's architecture.  If using the
   `--platform` build argument, it will be set automatically.
* `RUNNER_LISTEN_ADDRESS` - Disabled by default, setting this to the FQDN
  and port supports various observability and debugging features of the
  gitlab runner.  For more information see the [gitlab runner advanced
  configuration documentation](https://docs.gitlab.com/runner/configuration/advanced-configuration.html#the-global-section).
* `PRIVILEGED_RUNNER` - Defaults to 'false', may be set 'true'.  When
  `true`, this causes inner-containers to be created with the `--privileged`
  flag.  This is a potential security weakness, but is necessary for
  (among other things) allowing nested container image builds.
* `RUNNER_TAGS` - Defaults to `podman_in_podman`, may be set to any comma-separated
  list (with no spaces!) of tags.  These show up in GitLab (not the runner
  configuration), and determines where jobs are run.
* `RUNNER_UNTAGED` - Defaults to `true`, may be set to `false`.  Allows
  the runner to service jobs without any tags on them at all.
