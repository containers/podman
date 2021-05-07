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

## EXAMPLES

```
$ podman secret ls
$ podman secret ls --format "{{.Name}}"
```

## SEE ALSO
podman-secret(1)

## HISTORY
January 2021, Originally compiled by Ashley Cui <acui@redhat.com>
