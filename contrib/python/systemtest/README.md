![PODMAN logo](../../logo/podman-logo-source.svg)
# System Tests

Exercises system-level testing using already built, installed, and read-to-go podman.

## Prerequisites

* Podman and any dependencies are installed with the default configuration for the platform.
* The current user is root, or has password-less sudo access to root
* ``make`` is installed
* ``Python >= 3.4`` is installed

## Running the system-tests

From inside the top-level of the podman repository matching the installation, run:

```sh
make systemtest
```


## Running the system-test's unittests

All code shared across the system tests should have dedicated unittests.  They are run as part of
the top-level 'test' target.

```sh
make test
```
