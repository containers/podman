% podman-manifest(1)

## NAME
podman\-manifest - Create and manipulate manifest lists and image indexes

## SYNOPSIS
**podman manifest** *subcommand*

## DESCRIPTION
The `podman manifest` command provides subcommands which can be used to:

    * Create a working Docker manifest list or OCI image index.

## SUBCOMMANDS

| Command  | Man Page                                                     | Description                                                                 |
| -------- | ------------------------------------------------------------ | --------------------------------------------------------------------------- |
| add      | [podman-manifest-add(1)](podman-manifest-add.1.md)           | Add an image to a manifest list or image index.                             |
| annotate | [podman-manifest-annotate(1)](podman-manifest-annotate.1.md) | Add or update information about an entry in a manifest list or image index. |
| create   | [podman-manifest-create(1)](podman-manifest-create.1.md)     | Create a manifest list or image index.                                      |
| exists   | [podman-manifest-exists(1)](podman-manifest-exists.1.md)     | Check if the given manifest list exists in local storage                    |
| inspect  | [podman-manifest-inspect(1)](podman-manifest-inspect.1.md)   | Display a manifest list or image index.                                     |
| push     | [podman-manifest-push(1)](podman-manifest-push.1.md)         | Push a manifest list or image index to a registry.                          |
| remove   | [podman-manifest-remove(1)](podman-manifest-remove.1.md)     | Remove an image from a manifest list or image index.                        |
| rm       | [podman-manifest-rme(1)](podman-manifest-rm.1.md)            | Remove manifest list or image index from local storage.                |

## SEE ALSO
podman(1), podman-manifest-add(1), podman-manifest-annotate(1), podman-manifest-create(1), podman-manifest-inspect(1), podman-manifest-push(1), podman-manifest-remove(1)
