% podman-pod-logs(1)

## NAME
podman\-pod\-logs - Displays logs for pod with one or more containers

## SYNOPSIS
**podman pod logs** [*options*] *pod*

## DESCRIPTION
The podman pod logs command batch-retrieves whatever logs are present with all the containers of a pod. Pod logs can be filtered by container name or id using flag **-c** or **--container** if needed.

Note: Long running command of `podman pod log` with a `-f` or `--follow` needs to be reinvoked if new container is added to the pod dynamically otherwise logs of newly added containers would not be visible in log stream.

## OPTIONS

#### **--color**

Output the containers with different colors in the log.

#### **--container**, **-c**

By default `podman pod logs` retrieves logs for all the containers available within the pod differentiate by field `container`. However there are use-cases where user would want to limit the log stream only to a particular container of a pod for such cases `-c` can be used like `podman pod logs -c ctrNameorID podname`.

#### **--follow**, **-f**

Follow log output.  Default is false.

Note: If you are following a pod which is removed `podman pod rm`, then there is a
chance that the log file will be removed before `podman pod logs` reads the final content.

#### **--latest**, **-l**

Instead of providing the pod name or id, get logs of the last created pod. (This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)

#### **--names**, **-n**

Output the container names instead of the container IDs in the log.

#### **--since**=*TIMESTAMP*

Show logs since TIMESTAMP. The --since option can be Unix timestamps, date formatted timestamps, or Go duration
strings (e.g. 10m, 1h30m) computed relative to the client machine's time. Supported formats for date formatted
time stamps include RFC3339Nano, RFC3339, 2006-01-02T15:04:05, 2006-01-02T15:04:05.999999999, 2006-01-02Z07:00,
and 2006-01-02.

#### **--tail**=*LINES*

Output the specified number of LINES at the end of the logs.  LINES must be an integer.  Defaults to -1,
which prints all lines

#### **--timestamps**, **-t**

Show timestamps in the log outputs.  The default is false

#### **--until**=*TIMESTAMP*

Show logs until TIMESTAMP. The --until option can be Unix timestamps, date formatted timestamps, or Go duration
strings (e.g. 10m, 1h30m) computed relative to the client machine's time. Supported formats for date formatted
time stamps include RFC3339Nano, RFC3339, 2006-01-02T15:04:05, 2006-01-02T15:04:05.999999999, 2006-01-02Z07:00,
and 2006-01-02.

## EXAMPLE

To view a pod's logs:
```
podman pod logs -t podIdorName
```

To view logs of a specific container on the pod
```
podman pod logs -c ctrIdOrName podIdOrName
```

To view all pod logs:
```
podman pod logs -t --since 0 myserver-pod-1
```

To view a pod's logs since a certain time:
```
podman pod logs -t --since 2017-08-07T10:10:09.055837383-04:00 myserver-pod-1
```

To view a pod's logs generated in the last 10 minutes:
```
podman pod logs --since 10m myserver-pod-1
```

To view a pod's logs until 30 minutes ago:
```
podman pod logs --until 30m myserver-pod-1
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-pod(1)](podman-pod.1.md)**, **[podman-pod-rm(1)](podman-pod-rm.1.md)**, **[podman-logs(1)](podman-logs.1.md)**
