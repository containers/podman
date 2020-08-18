![PODMAN logo](../../logo/podman-logo-source.svg)

The "gate" image is a standard container image for lint-checking and validating
changes to the libpod repository.  It must be built from the repository root as
[described in the contibutors guide](https://github.com/containers/podman/blob/master/CONTRIBUTING.md#go-format-and-lint).
The image is also used in [CI/CD automation](../../.cirrus.yml).
