# Podman Codebase structure

Description about important directories in our repository.

### cmd/

 - each directory here builds its own binary

#### cmd/podman/

- podman cli code, cli commands and flags are defined here, we are using github.com/spf13/cobra as library for command line parsing

#### cmd/quadlet/

- quadlet cli

### contrib/

- CI scripts, packaging files some container image build files

### docs/

- sphinx based documentation for podman that is build on readthedocs (docs.podman.io)

#### docs/source/markdown/

- man pages for each podman command

### libpod/ (only works on linux and freebsd)
- underlying core for most podman operations, defines container, pod, volume management opartions
- contains the database to store these information on disk, either sqlite or botldb (our old db format)
- integrates with our other libraries such as:
	- containers/storage create and mount container storage
	- containers/buildah for building images
	- containers/common/libnetwork for network management

### pkg/

- various packages to do all sorts of different things

#### pkg/api/

- the HTTP REST API server code

#### pkg/bindings/

- the HTTP REST API client code
- this package must have a stable API as it is for use by external consumers as well

#### pkg/domain/

- "glue" code between cli and the actual operations performed

##### pkg/domain/entities/

- the package defines two interfaces (ContainerEngine, ImageEngine) that more or less have a function for each cli command defined
- it also defines a lot of types (structs) for the various options the functions accept

##### pkg/domain/infra/tunnel/

- implements the two interfaces for the remote mode (podman-remote) which just maps each operations to the bindings code from pkg/bindings which then talks to the remote server

##### pkg/domain/infra/abi/

- implements the two interfaces for the local mode (podman) that calls then directly into the core parts of libpod/ or our other libraries containers/{common,image,storage}...

#### pkg/libartifact/

- core code for the new podman artifact commands

#### pkg/machine/

- core code for podman machine commands

##### pkg/machine/e2e/

- e2e tests for podman machine commands
- runs on windows, macos, linux.

### test/

- various tests suites, see the test [README.md](../test/README.md) for more details
- these run on linux only

#### vendor/

- directory created with "go mod vendor"
- this includes all go deps in our repo, DO NOT edit this directory directly, changes in dependencies must be made in their respective upstream repositories and then updated in go.mod

#### bin/

- build binaries are put here (bin/podman, bin/podman-remote, etc...)
