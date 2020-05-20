% podman-network-inspect(1)

## NAME
podman\-network\-inspect - Displays the raw CNI network configuration for one or more networks

## SYNOPSIS
**podman network inspect**  [*network* ...]

## DESCRIPTION
Display the raw (JSON format) network configuration. This command is not available for rootless users.

## OPTIONS
**--quiet**, **-q**

The `quiet` option will restrict the output to only the network names.

**--format**, **-f**

Pretty-print networks to JSON or using a Go template.

## EXAMPLE

Inspect the default podman network

```
# podman network inspect podman
[{
    "cniVersion": "0.3.0",
    "name": "podman",
    "plugins": [
      {
        "type": "bridge",
        "bridge": "cni0",
        "isGateway": true,
        "ipMasq": true,
        "ipam": {
            "type": "host-local",
            "subnet": "10.88.1.0/24",
            "routes": [
                { "dst": "0.0.0.0/0" }
            ]
        }
      },
      {
        "type": "portmap",
        "capabilities": {
          "portMappings": true
        }
      }
    ]
}
]
```

```
# podman network inspect podman --format '{{(index  .plugins  0).ipam.ranges}}'
[[map[gateway:10.88.0.1 subnet:10.88.0.0/16]]]
```

## SEE ALSO
podman(1), podman-network(1), podman-network-ls(1)

## HISTORY
August 2019, Originally compiled by Brent Baude <bbaude@redhat.com>
