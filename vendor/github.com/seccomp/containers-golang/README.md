`containers-golang` is a set of Go libraries used by container runtimes to generate and load seccomp mappings into the kernel.

seccomp (short for secure computing mode) is a BPF based syscall filter language and present a more conventional function-call based filtering interface that should be familiar to, and easily adopted by, application developers.

## Building
   make - Generates default.json file, which containes the whitelisted syscalls that can be used by container runtime engines like [CRI-O][cri-o], [Buildah][buildah], [Podman][podman] and [Docker][docker], and container runtimes like OCI [Runc][runc] to controll the syscalls available to containers.

### Supported build tags

   `seccomp`
   
## Contributing

When developing this library, please use `make` (or `make … BUILDTAGS=…`) to take advantage of the tests and validation.

## License

ASL 2.0

## Contact

- IRC: #[CRI-O](irc://irc.freenode.net:6667/#cri-o) on freenode.net

[cri-o]:   https://github.com/kubernetes-incubator/cri-o/pulls
[buildah]: https://github.com/projectatomic/buildah
[podman]:  https://github.com/projectatomic/podman
[docker]:  https://github.com/docker/docker
[runc]:    https://github.com/opencontainers/runc

