% podman-network-inspect(1)

## NAME
podman\-network\-inspect - Displays the raw CNI network configuration for one or more networks

## SYNOPSIS
**podman network inspect**  [*network* ...]

## DESCRIPTION
Display the raw (JSON format) network configuration. This command is not available for rootless users.

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

## SEE ALSO
podman(1), podman-network(1), podman-network-ls(1)

## HISTORY
August 2019, Originally compiled by Brent Baude <bbaude@redhat.com>
