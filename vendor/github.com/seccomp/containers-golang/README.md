`containers-golang` is a set of Go libraries used by container runtimes to generate and load seccomp mappings into the kernel.

seccomp (short for secure computing mode) is a computer security facility in the Linux kernel. It was merged into the Linux kernel mainline in kernel version 2.6.12, which was released on March 8, 2005.[1] seccomp allows a process to make a one-way transition into a "secure" state where it cannot make any system calls except exit(), sigreturn(), read() and write() to already-open file descriptors. Should it attempt any other system calls, the kernel will terminate the process with SIGKILL or SIGSYS[2][3]. In this sense, it does not virtualize the system's resources but isolates the process from them entirely.

## Dependencies

## Building

### Supported build tags

## Contributing

When developing this library, please use `make` (or `make … BUILDTAGS=…`) to take advantage of the tests and validation.

## License

ASL 2.0

## Contact

- IRC: #[CRI-O](irc://irc.freenode.net:6667/#cri-o) on freenode.net
