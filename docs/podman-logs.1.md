% podman-logs(1)

## NAME
podman\-logs - Fetch the logs of a container

## SYNOPSIS
**podman** **logs** [*options*] *container*

## DESCRIPTION
The podman logs command batch-retrieves whatever logs are present for a container at the time of execution.
This does not guarantee execution order when combined with podman run (i.e. your run may not have generated
any logs at the time you execute podman logs

## OPTIONS

**--follow, -f**

Follow log output.  Default is false

**--since=TIMESTAMP**

Show logs since TIMESTAMP. The --since option can be Unix timestamps, date formatted timestamps, or Go duration
strings (e.g. 10m, 1h30m) computed relative to the client machine's time. Supported formats for date formatted
time stamps include RFC3339Nano, RFC3339, 2006-01-02T15:04:05, 2006-01-02T15:04:05.999999999, 2006-01-02Z07:00,
and 2006-01-02.

**--tail=LINES**

Output the specified number of LINES at the end of the logs.  LINES must be a positive integer.  Defaults to 0,
which prints all lines

**--timestamps, -t**

Show timestamps in the log outputs.  The default is false

## EXAMPLE

To view a container's logs:
```
podman logs b3f2436bdb978c1d33b1387afb5d7ba7e3243ed2ce908db431ac0069da86cb45

2017/08/07 10:16:21 Seeked /var/log/crio/pods/eb296bd56fab164d4d3cc46e5776b54414af3bf543d138746b25832c816b933b/c49f49788da14f776b7aa93fb97a2a71f9912f4e5a3e30397fca7dfe0ee0367b.log - &{Offset:0 Whence:0}
1:C 07 Aug 14:10:09.055 # oO0OoO0OoO0Oo Redis is starting oO0OoO0OoO0Oo
1:C 07 Aug 14:10:09.055 # Redis version=4.0.1, bits=64, commit=00000000, modified=0, pid=1, just started
1:C 07 Aug 14:10:09.055 # Warning: no config file specified, using the default config. In order to specify a config file use redis-server /path/to/redis.conf
1:M 07 Aug 14:10:09.055 # You requested maxclients of 10000 requiring at least 10032 max file descriptors.
1:M 07 Aug 14:10:09.055 # Server can't set maximum open files to 10032 because of OS error: Operation not permitted.
1:M 07 Aug 14:10:09.055 # Current maximum open files is 4096. maxclients has been reduced to 4064 to compensate for low ulimit. If you need higher maxclients increase 'ulimit -n'.
1:M 07 Aug 14:10:09.056 * Running mode=standalone, port=6379.
1:M 07 Aug 14:10:09.056 # WARNING: The TCP backlog setting of 511 cannot be enforced because /proc/sys/net/core/somaxconn is set to the lower value of 128.
1:M 07 Aug 14:10:09.056 # Server initialized
```

To view only the last two lines in container's log:
```
podman logs --tail 2 b3f2436bdb97

1:M 07 Aug 14:10:09.056 # WARNING: The TCP backlog setting of 511 cannot be enforced because /proc/sys/net/core/somaxconn is set to the lower value of 128.
1:M 07 Aug 14:10:09.056 # Server initialized
```

To view a containers logs since a certain time:
```
podman logs 224c375f27cd --since 2017-08-07T10:10:09.055837383-04:00 myserver

1:M 07 Aug 14:10:09.055 # Server can't set maximum open files to 10032 because of OS error: Operation not permitted.
1:M 07 Aug 14:10:09.055 # Current maximum open files is 4096. maxclients has been reduced to 4064 to compensate for low ulimit. If you need higher maxclients increase 'ulimit -n'.
1:M 07 Aug 14:10:09.056 * Running mode=standalone, port=6379.
1:M 07 Aug 14:10:09.056 # WARNING: The TCP backlog setting of 511 cannot be enforced because /proc/sys/net/core/somaxconn is set to the lower value of 128.
1:M 07 Aug 14:10:09.056 # Server initialized
```

To view a container's logs generated in the last 10 minutes:
```
podman logs 224c375f27cd --since 10m myserver

1:M 07 Aug 14:10:09.055 # Server can't set maximum open files to 10032 because of OS error: Operation not permitted.
1:M 07 Aug 14:10:09.055 # Current maximum open files is 4096. maxclients has been reduced to 4064 to compensate for low ulimit. If you need higher maxclients increase 'ulimit -n'.
```

## SEE ALSO
podman(1)

## HISTORY
August 2017, Originally compiled by Ryan Cole <rycole@redhat.com>
February 2018, Updated by Brent Baude <bbaude@redhat.com>
