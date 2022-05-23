% podman-secret-ls(1)

## NAME
podman\-secret\-ls - List all available secrets

## SYNOPSIS
**podman secret ls** [*options*]

## DESCRIPTION

Lists all the secrets that exist. The output can be formatted to a Go template using the **--format** option.

## OPTIONS

#### **--filter**, **-f**=*filter=value*

Filter output based on conditions given.
Multiple filters can be given with multiple uses of the --filter option.

Valid filters are listed below:

| **Filter** | **Description**                                                   |
| ---------- | ----------------------------------------------------------------- |
| name       | [Name] Secret name (accepts regex)                                |
| id         | [ID] Full or partial secret ID                                    |

#### **--format**=*format*

Format secret output using Go template.

| **Placeholder**     | **Description**    |
| ------------------- | ------------------ |
| .ID                 | ID of secret       |
| .Spec               | FIXME: this does not seem to work |
| .CreatedAt          | When secret was created (relative timestamp, human-readable) |
| .UpdatedAt          | When secret was last updated (relative timestamp, human-readable) |

#### **--noheading**

Omit the table headings from the listing of secrets.	.

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
