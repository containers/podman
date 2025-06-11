% podman-quadlet-list 1

## NAME
podman\-quadlet\-list - List installed quadlets

## SYNOPSIS
**podman quadlet list** [*options*]

## DESCRIPTION

List all Quadlets configured for the current user.

## OPTIONS

#### **--filter**, **-f**

Filter output based on conditions give.

#### **--format**

Pretty-print output to JSON or using a Go template (default "{{range .}}{{.Name}}\t{{.UnitName}}\t{{.Path}}\t{{.Status}}\n{{end -}}")

Print results with a Go template.

| **Placeholder** | **Description**                                |
|-----------------|------------------------------------------------|
| .Name           | Name of the Quadlet file                       |
| .Path           | Quadlet file path on disk                      |
| .Status         | Quadlet status corresponding to systemd unit   |
| .UnitName       | Systemd unit name corresponding to quadlet     |

## EXAMPLES

Filter list by name.

```
$ podman quadlet list --filter 'name=test*'
```

Format list output for a specific field.
```
$ podman quadlet list --format '{{ .Unit }}'
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-quadlet(1)](podman-quadlet.1.md)**
