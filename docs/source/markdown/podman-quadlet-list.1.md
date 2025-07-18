% podman-quadlet-list 1

## NAME
podman\-quadlet\-list - List installed quadlets

## SYNOPSIS
**podman quadlet list** [*options*]

## DESCRIPTION

List all Quadlets configured for the current user.

## OPTIONS

#### **--filter**, **-f**=*filter*

Filter output based on conditions give.

The *filters* argument format is of `key=value`. If there is more than one *filter*, then pass multiple OPTIONS: **--filter** *foo=bar* **--filter** *bif=baz*.

Supported filters:

| Filter     | Description                                                                                      |
|------------|--------------------------------------------------------------------------------------------------|
| name       | Filter by quadlet name.                                                                          |

#### **--format**=*format*

Pretty-print output to JSON or using a Go template (default "{{range .}}{{.Name}}\t{{.UnitName}}\t{{.Path}}\t{{.Status}}\n{{end -}}")

Print results with a Go template.

| **Placeholder** | **Description**                                  |
|-----------------|--------------------------------------------------|
| .App            | Name of application if Quadlet is part of an app |
| .Name           | Name of the Quadlet file                         |
| .Path           | Quadlet file path on disk                        |
| .Status         | Quadlet status corresponding to systemd unit     |
| .UnitName       | Systemd unit name corresponding to quadlet       |

## EXAMPLES

Simple list command

```
$ podman quadlet list
NAME                            UNIT NAME                     PATH ON DISK                                                          STATUS      APPLICATION
test-service-quadlet.container  test-service-quadlet.service  /home/user/.config/containers/systemd/test-service-quadlet.container  Not loaded
sample-quadlet.container        sample-quadlet.service        /home/user/.config/containers/systemd/sample-quadlet.container        Not loaded
```


Filter list by name.

```
$ podman quadlet list --filter 'name=test*'
NAME                            UNIT NAME                     PATH ON DISK                                                          STATUS      APPLICATION
test-service-quadlet.container  test-service-quadlet.service  /home/user/.config/containers/systemd/test-service-quadlet.container  Not loaded
```

Format list output for a specific field.
```
$ podman quadlet list --format '{{ .UnitName }}'
UNIT NAME
test-service-quadlet.service
sample-quadlet.service
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-quadlet(1)](podman-quadlet.1.md)**
