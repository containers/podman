[![GoDoc](https://godoc.org/github.com/containers/psgo?status.svg)](https://godoc.org/github.com/containers/psgo/ps) [![Build Status](https://travis-ci.org/containers/psgo.svg?branch=master)](https://travis-ci.org/containers/psgo)

# psgo
A ps (1) AIX-format compatible golang library.  Please note, that the library is still under development.

The idea behind the library is to implement an easy to use way of extracting process-related data, just as ps (1) does. The problem when using ps (1) is that the ps format strings split columns with whitespaces, making the output nearly impossible to parse. It also adds some jitter as we have to fork.

This library aims to make things a bit more comfortable, especially for container runtimes, as the API allows to join the mount namespace of a given process and will parse `/proc` from there. Currently, the API consists of two functions:

 - `ProcessInfo(format string) ([]string, error)`
   - ProcessInfo returns the process information of all processes in the current mount namespace. The input format must be a comma-separated list of supported AIX format descriptors.  If the input string is empty, the DefaultFormat is used. The return value is an array of tab-separated strings, to easily use the output for column-based formatting (e.g., with the `text/tabwriter` package).

 - `JoinNamespaceAndProcessInfo(pid, format string) ([]string, error)`
   - JoinNamespaceAndProcessInfo has the same semantics as ProcessInfo but joins the mount namespace of the specified pid before extracting data from /proc.  This way, we can extract the `/proc` data from a container without executing any command inside the container.

A sample implementation using this API can be found [here](https://github.com/containers/psgo/blob/master/psgo.go). You can compile the sample `psgo` tool via `make build`.

### Listing processes
```
./bin/psgo | head -n5
USER         PID     PPID    %CPU     ELAPSED              TTY      TIME        COMMAND
root         1       0       0.064    6h3m27.677997443s    ?        13.98s      systemd
root         2       0       0.000    6h3m27.678380128s    ?        20ms        [kthreadd]
root         4       2       0.000    6h3m27.678701852s    ?        0s          [kworker/0:0H]
root         6       2       0.000    6h3m27.678999508s    ?        0s          [mm_percpu_wq]
```

### Changing the output format
The format strings are ps (1) AIX format strings, and must be separated by commas:
```
CODE   NORMAL   HEADER
%C     pcpu     %CPU
%G     group    GROUP
%P     ppid     PPID
%U     user     USER
%a     args     COMMAND
%c     comm     COMMAND
%g     rgroup   RGROUP
%n     nice     NI
%p     pid      PID
%r     pgid     PGID
%t     etime    ELAPSED
%u     ruser    RUSER
%x     time     TIME
%y     tty      TTY
%z     vsz      VSZ
```

To extract the effective user ID, the PID and and the command (i.e., name of the binary), we can run `./bin/psgo -format "user, %p, comm"`. Notice, that both, the *code* and *normal* notation of the descriptors can be used.

### List processes inside a container / Joining another mount namespace
To demonstrate the usecase for containers, let's run a container and display the running processes inside this container:

```
$ docker run -d --name foo alpine sleep 100
$ docker inspect --format '{{.State.Pid}}' foo
$ sudo ./bin/psgo -pid1377
USER   PID   PPID   %CPU    ELAPSED         TTY   TIME   COMMAND
root   1     0      0.193   25.959923679s   ?     50ms   sleep
```
