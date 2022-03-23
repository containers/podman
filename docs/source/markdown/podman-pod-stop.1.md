% podman-pod-stop(1)

## NAME
podman\-pod\-stop - Stop one or more pods

## SYNOPSIS
**podman pod stop** [*options*] *pod* ...

## DESCRIPTION
Stop containers in one or more pods.  You may use pod IDs or names as input.

## OPTIONS

#### **--all**, **-a**

Stops all pods

#### **--ignore**, **-i**

Ignore errors when specified pods are not in the container store.  A user might
have decided to manually remove a pod which would lead to a failure during the
ExecStop directive of a systemd service referencing that pod.

#### **--latest**, **-l**

Instead of providing the pod name or ID, stop the last created pod. (This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)

#### **--pod-id-file**

Read pod ID from the specified file and stop the pod.  Can be specified multiple times.

#### **--time**, **-t**=*seconds*

Seconds to wait before forcibly stopping the containers in the pod.

## EXAMPLE

Stop a pod called *mywebserverpod*
```
$ podman pod stop mywebserverpod
cc8f0bea67b1a1a11aec1ecd38102a1be4b145577f21fc843c7c83b77fc28907
```

Stop two pods by their short IDs.
```
$ podman pod stop 490eb 3557fb
490eb241aaf704d4dd2629904410fe4aa31965d9310a735f8755267f4ded1de5
3557fbea6ad61569de0506fe037479bd9896603c31d3069a6677f23833916fab
```

Stop the most recent pod
```
$ podman pod stop --latest
3557fbea6ad61569de0506fe037479bd9896603c31d3069a6677f23833916fab
```

Stop all pods
```
$ podman pod stop --all
19456b4cd557eaf9629825113a552681a6013f8c8cad258e36ab825ef536e818
3557fbea6ad61569de0506fe037479bd9896603c31d3069a6677f23833916fab
490eb241aaf704d4dd2629904410fe4aa31965d9310a735f8755267f4ded1de5
70c358daecf71ef9be8f62404f926080ca0133277ef7ce4f6aa2d5af6bb2d3e9
cc8f0bea67b1a1a11aec1ecd38102a1be4b145577f21fc843c7c83b77fc28907
```

Stop two pods via --pod-id-file
```
$ podman pod stop --pod-id-file file1 --pod-id-file file2
19456b4cd557eaf9629825113a552681a6013f8c8cad258e36ab825ef536e818
cc8f0bea67b1a1a11aec1ecd38102a1be4b145577f21fc843c7c83b77fc28907
```

Stop all pods with a timeout of 1 second.
```
$ podman pod stop -a -t 1
3557fbea6ad61569de0506fe037479bd9896603c31d3069a6677f23833916fab
490eb241aaf704d4dd2629904410fe4aa31965d9310a735f8755267f4ded1de5
70c358daecf71ef9be8f62404f926080ca0133277ef7ce4f6aa2d5af6bb2d3e9
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-pod(1)](podman-pod.1.md)**, **[podman-pod-start(1)](podman-pod-start.1.md)**

## HISTORY
July 2018, Originally compiled by Peter Hunt <pehunt@redhat.com>
