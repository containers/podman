[![GoDoc](https://godoc.org/github.com/containers/psgo?status.svg)](https://godoc.org/github.com/containers/psgo/ps) [![Build Status](https://travis-ci.org/containers/psgo.svg?branch=master)](https://travis-ci.org/containers/psgo)

# psgo
A ps (1) AIX-format compatible golang library extended with various descriptors useful for displaying container-related data.

The idea behind the library is to provide an easy to use way of extracting process-related data, just as ps (1) does. The problem when using ps (1) is that the ps format strings split columns with whitespaces, making the output nearly impossible to parse. It also adds some jitter as we have to fork and execute ps either in the container or filter the output afterwards, further limiting applicability.

This library aims to make things a bit more comfortable, especially for container runtimes, as the API allows to join the mount namespace of a given process and will parse `/proc` and `/dev/` from there. The API consists of the following functions:

 - `ps.ProcessInfo(format string) ([]string, error)`
   - ProcessInfo returns the process information of all processes in the current mount namespace. The input format must be a comma-separated list of supported AIX format descriptors.  If the input string is empty, the DefaultFormat is used. The return value is a slice of tab-separated strings, to easily use the output for column-based formatting (e.g., with the `text/tabwriter` package).

 - `ps.JoinNamespaceAndProcessInfo(pid, format string) ([]string, error)`
   - JoinNamespaceAndProcessInfo has the same semantics as ProcessInfo but joins the mount namespace of the specified pid before extracting data from /proc.  This way, we can extract the `/proc` data from a container without executing any command inside the container.

 - `ps.ListDescriptors() []string`
   - ListDescriptors returns a sorted string slice of all supported AIX format descriptors in the normal form (e.g., "args,comm,user").  It can be useful in the context of bash-completion, help messages, etc.

### Listing processes
We can use the [psgo](https://github.com/containers/psgo/blob/master/psgo.go) tool from this project to test the core components of this library. First, let's build `psgo` via `make build`.  The binary is now located under `./bin/psgo`.  By default `psgo` displays data about all running processes in the current mount namespace, similar to the output of `ps -ef`.

```
$ ./bin/psgo | head -n5
USER         PID     PPID    %CPU     ELAPSED              TTY      TIME        COMMAND
root         1       0       0.064    6h3m27.677997443s    ?        13.98s      systemd
root         2       0       0.000    6h3m27.678380128s    ?        20ms        [kthreadd]
root         4       2       0.000    6h3m27.678701852s    ?        0s          [kworker/0:0H]
root         6       2       0.000    6h3m27.678999508s    ?        0s          [mm_percpu_wq]
```

### Listing processes within a container
Let's have a look at how we can use this library in the context of containers.  As a simple show case, we'll start a Docker container, extract the process ID via `docker-inspect` and run the `psgo` binary to extract the data of running processes within that container.

```shell
$ docker run -d alpine sleep 100
473c9a05d4223b88ef7f5a9ac11e3d21e9914e012338425cc1cef853fc6c32a2

$ docker inspect --format '{{.State.Pid}}' 473c9
5572

$ sudo ./bin/psgo -pid 5572
USER   PID   PPID   %CPU    ELAPSED         TTY   TIME   COMMAND
root   1     0      0.000   17.249905587s   ?     0s     sleep
```

### Format descriptors
The ps library is compatible with all AIX format descriptors of the ps command-line utility (see `man 1 ps` for details) but it also supports some additional descriptors that can be useful when seeking specific process-related information.

- **capinh**
  - Set of inheritable capabilities. See capabilities (7) for more information.
- **capprm**
  - Set of permitted capabilities. See capabilities (7) for more information.
- **capeff**
  - Set of effective capabilities. See capabilities (7) for more information.
- **capbnd**
  - Set of bounding capabilities. See capabilities (7) for more information.
- **seccomp**
  - Seccomp mode of the process (i.e., disabled, strict or filter). See seccomp (2) for more information.
- **label**
  - Current security attributes of the process.

We can try out different format descriptors with the psgo binary:

```shell
$ ./bin/psgo -format "pid, user, group, seccomp" | head -n5
PID     USER         GROUP        SECCOMP
1       root         root         disabled
2       root         root         disabled
4       root         root         disabled
6       root         root         disabled
```
