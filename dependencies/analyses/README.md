# A set of scripts and instructions that help to analyze and debloat go-lang dependencies

Note that all scripts mentioned below follow the [KISS principle](https://en.wikipedia.org/wiki/KISS_principle) on purpose.
The scripts are meant to be used in combination to aid in understanding the package's dependencies and how they contribute to the size of the compiled binary.

## Size of packages

To analyze the size of all go packages used during the build process, pass the `-work -a` build flags to `go build`.
The `-a` flag forces go to rebuild all packages even if they are already up-to-date (e.g., in the build cache), while the `-work` flag instructs go to print the temporary work directory used for compiling the packages.
The path to the temporary work directory of `go-build` must be passed to `go-archive-analysis.sh` by setting it as an environment variable.
The analysis script will then read and parse the build data and print a sorted table of the package size in bytes followed by the package name.

Running such an analysis on libpod may look as follows:

```
# 1) Build the Podman binary with `-work -a`.
[libpod]$ BUILDFLAGS="-work -a" make podman
[...]
WORK=/tmp/go-build794287815

# 2) Set the work directory as an environment variable and call the analysis script
[libpod]$ WORK=/tmp/go-build794287815 ./dependencies/analyses/go-archive-analysis.sh | head -n10
17M github.com/containers/podman/cmd/podman/cliconfig
13M github.com/containers/podman/vendor/github.com/DataDog/zstd
10M github.com/containers/podman/vendor/k8s.io/api/core/v1
3.7M net/http
3.7M github.com/containers/podman/libpod
3.2M runtime
2.7M github.com/containers/podman/vendor/github.com/gogo/protobuf/proto
2.5M github.com/containers/podman/vendor/k8s.io/apimachinery/pkg/apis/meta/v1
2.3M github.com/containers/podman/vendor/github.com/vishvananda/netlink
```

The output of the `go-archive-analysis.sh` script is a sorted table with the size in bytes followed by the package.
The size denotes the size of the compiled package (i.e., the `.a` file).


## Size of symbols in binary

Once the binary is compiled, we can run another set of analyses on it.
The `nm-symbols-analysis.sh` is a wrapper around `go tool nm` and prints a table with the size in bytes followed by the symbol's name.
To avoid information overload, the scripts prints only symbols from the text/code segment.

Running such an analysis on libpod may look as follows:

```
# 1) Compile the binary
[libpod]$ make podman
[...]

# 2) Run the script with the binary as an argument
[libpod]$ ./dependencies/analyses/nm-symbols-analysis.sh ./bin/podman | grep "containers/libpod/libpod" | head -n10
299             github.com/containers/podman/libpod.(*BoltState).AddContainer
658             github.com/containers/podman/libpod.(*BoltState).AddContainerToPod
2120            github.com/containers/podman/libpod.(*BoltState).AddPod
3773            github.com/containers/podman/libpod.(*BoltState).AddPod.func1
965             github.com/containers/podman/libpod.(*BoltState).AddVolume
1651            github.com/containers/podman/libpod.(*BoltState).AddVolume.func1
558             github.com/containers/podman/libpod.(*BoltState).AllContainers
282             github.com/containers/podman/libpod.(*BoltState).AllContainers.func1
1121            github.com/containers/podman/libpod.(*BoltState).AllContainers.func1.1
558             github.com/containers/podman/libpod.(*BoltState).AllPods
```

Running the script can help identify sources of bloat and reveal potential candidates (e.g., entire packages, types, or function) for refactoring.


## Dependency Tree

Use the `dependency-tree.sh` script to figure out which package includes which packages.
The output of the script has the format `package: dependency_1, dependency_2, ...`.
Each line is followed by a blank line to make it easier to read.
The script generates two files:

 - `direct-tree.txt` - listing direct dependencies
 - `transitive-tree.txt` - listing direct and transitive dependencies

Running such a dependency-tree analysis may look as follows:


```
[libpod]$ ./dependencies/analyses/dependency-tree.sh github.com/containers/podman
[libpod]$ grep "^github.com/containers/podman/pkg/registries" direct-tree.txt
github.com/containers/podman/pkg/registries: github.com/containers/podman/vendor/github.com/containers/image/pkg/sysregistriesv2, github.com/containers/podman/vendor/github.com/containers/image/types, github.com/containers/podman/pkg/rootless, github.com/containers/podman/vendor/github.com/docker/distribution/reference, github.com/containers/podman/vendor/github.com/pkg/errors, os, path/filepath, strings
```

As shown above, the script's output can then be used to query for specific packages (e.g, with `grep`).
