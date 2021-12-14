% podman-secret-ls(1)

## NAME
podman\-secret\-ls - List all available secrets

## SYNOPSIS
**podman secret ls** [*options*]

## DESCRIPTION

Lists all the secrets that exist. The output can be formatted to a Go template using the **--format** option.

## OPTIONS

#### **--format**=*format*

Format secret output using Go template.

#### **--noheading**

Omit the table headings from the listing of secrets.	.

#### **--filter**, **-f**=*filter=value*

Filter output based on conditions given.
Multiple filters can be given with multiple uses of the --filter option.

Valid filters are listed below:

| **Filter** | **Description**                                                   |
| ---------- | ----------------------------------------------------------------- |
| name       | [Name] Secret name (accepts regex)                                |
| id         | [ID] Full or partial secret ID                                    |

## EXAMPLES

```
$ podman secret ls
$ podman secret ls --format "{{.Name}}"
$ podman secret ls --filter name=confidential
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-secret(1)](podman-secret.1.md)**

## HISTORY
January 2021, Originally compiled by Ashley Cui <acui@redhat.com>
