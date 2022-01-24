% podman-machine-ls(1)

## NAME
podman\-machine\-list - List virtual machines

## SYNOPSIS
**podman machine list** [*options*]

**podman machine ls** [*options*]

## DESCRIPTION

List Podman managed virtual machines.

Podman on macOS requires a virtual machine. This is because containers are Linux -
containers do not run on any other OS because containers' core functionality is
tied to the Linux kernel.

## OPTIONS

#### **--format**=*format*

Change the default output format.  This can be of a supported type like 'json'
or a Go template.
Valid placeholders for the Go template are listed below:

| **Placeholder** | **Description**                 |
| --------------- | ------------------------------- |
| .CPUs           | Number of CPUs                  |
| .Created        | Time since VM creation          |
| .Default        | Is default machine              |
| .DiskSize       | Disk size of machine            |
| .LastUp         | Time machine was last up        |
| .LastUp         | Time since the VM was last run  |
| .Memory         | Allocated memory for machine   |
| .Name           | VM name                         |
| .Running        | Is machine running              |
| .Stream         | Stream name                     |
| .VMType         | VM type                         |
| .Port           | SSH Port to use to connect to VM|
| .RemoteUsername | VM Username for rootless Podman |
| .IdentityPath   | Path to ssh identity file       |

#### **--help**

Print usage statement.

#### **--noheading**

Omit the table headings from the listing of pods.

## EXAMPLES

```
$ podman machine list
NAME                    VM TYPE     CREATED      LAST UP      CPUS        MEMORY      DISK SIZE
podman-machine-default  qemu        2 weeks ago  2 weeks ago  1           2.147GB     10.74GB

$ podman machine ls --format "table {{.Name}}\t{{.VMType}}\t{{.Created}}\t{{.LastUp}}"
NAME                    VM TYPE     CREATED      LAST UP
podman-machine-default  qemu        2 weeks ago  2 weeks ago

$ podman machine ls --format json
[
    {
        "Name": "podman-machine-default",
        "Default": false,
        "Created": "2021-12-27T10:36:14.373347492-05:00",
        "Running": false,
        "LastUp": "2021-12-27T11:22:50.17333371-05:00",
        "Stream": "default",
        "VMType": "qemu",
        "CPUs": 1,
        "Memory": "2147483648",
        "DiskSize": "10737418240"
    }
]
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-machine(1)](podman-machine.1.md)**

## HISTORY
March 2021, Originally compiled by Ashley Cui <acui@redhat.com>
