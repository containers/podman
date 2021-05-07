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

Format list output using a Go template.

Valid placeholders for the Go template are listed below:

| **Placeholder** | **Description**                 |
| --------------- | ------------------------------- |
| .Name           | VM name                         |
| .Created        | Time since VM creation          |
| .LastUp         | Time since the VM was last run  |
| .VMType         | VM type                      	|

#### **--help**

Print usage statement.

#### **--noheading**

Omit the table headings from the listing of pods.

## EXAMPLES

```
$ podman machine list

$ podman machine ls --format {{.Name}}\t{{.VMType}}\t{{.Created}}\t{{.LastUp}}\n
```

## SEE ALSO
podman-machine(1)

## HISTORY
March 2021, Originally compiled by Ashley Cui <acui@redhat.com>
