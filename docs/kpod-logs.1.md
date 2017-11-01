% kpod(1) kpod-logs - Fetch the logs of a container
% Ryan Cole
# kpod-logs "1" "March 2017" "kpod"

## NAME
kpod logs - Fetch the logs of a container

## SYNOPSIS
**kpod** **logs** [*options* [...]] container

## DESCRIPTION
The kpod logs command batch-retrieves whatever logs are present for a container at the time of execution.  This does not guarantee execution order when combined with kpod run (i.e. your run may not have generated any logs at the time you execute kpod logs

## OPTIONS

**--follow, -f**

Follow log output.  Default is false

**--since=TIMESTAMP**

Show logs since TIMESTAMP

**--tail=LINES**

Ouput the specified number of LINES at the end of the logs.  LINES must be a positive integer.  Defaults to 0, which prints all lines

## EXAMPLE

kpod logs b3f2436bdb978c1d33b1387afb5d7ba7e3243ed2ce908db431ac0069da86cb45

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


kpod logs --tail 2 b3f2436bdb97

1:M 07 Aug 14:10:09.056 # WARNING: The TCP backlog setting of 511 cannot be enforced because /proc/sys/net/core/somaxconn is set to the lower value of 128.
1:M 07 Aug 14:10:09.056 # Server initialized

kpod logs 224c375f27cd --since 2017-08-07T10:10:09.055837383-04:00 myserver

1:M 07 Aug 14:10:09.055 # Server can't set maximum open files to 10032 because of OS error: Operation not permitted.
1:M 07 Aug 14:10:09.055 # Current maximum open files is 4096. maxclients has been reduced to 4064 to compensate for low ulimit. If you need higher maxclients increase 'ulimit -n'.
1:M 07 Aug 14:10:09.056 * Running mode=standalone, port=6379.
1:M 07 Aug 14:10:09.056 # WARNING: The TCP backlog setting of 511 cannot be enforced because /proc/sys/net/core/somaxconn is set to the lower value of 128.
1:M 07 Aug 14:10:09.056 # Server initialized

## SEE ALSO
kpod(1)

## HISTORY
August 2017, Originally compiled by Ryan Cole <rycole@redhat.com>
