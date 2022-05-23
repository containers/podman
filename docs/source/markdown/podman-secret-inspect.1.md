% podman-secret-inspect(1)

## NAME
podman\-secret\-inspect - Display detailed information on one or more secrets

## SYNOPSIS
**podman secret inspect** [*options*] *secret* [...]

## DESCRIPTION

Inspects the specified secret.

By default, this renders all results in a JSON array. If a format is specified, the given template will be executed for each result.
Secrets can be queried individually by providing their full name or a unique partial name.

## OPTIONS

#### **--format**=*format*

Format secret output using Go template.

| **Placeholder**          | **Description**                                                   |
| ------------------------ | ----------------------------------------------------------------- |
| .ID                      | ID of secret                                                      |
| .Spec                    | Details of secret                                                 |
| .Spec.Name               | Name of secret                                                    |
| .Spec.Driver             | Driver info                                                       |
| .Spec.Driver.Name        | Driver name (string)                                              |
| .Spec.Driver.Options ... | Driver options (map of driver-specific options)                   |
| .CreatedAt               | When secret was created (relative timestamp, human-readable)      |
| .UpdatedAt               | When secret was last updated (relative timestamp, human-readable) |

#### **--help**

Print usage statement.


## EXAMPLES

```
$ podman secret inspect mysecret
$ podman secret inspect --format "{{.Name} {{.Scope}}" mysecret
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-secret(1)](podman-secret.1.md)**

## HISTORY
January 2021, Originally compiled by Ashley Cui <acui@redhat.com>
