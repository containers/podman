% podman-image-trust "1"

# NAME
podman\-trust - Manage container registry image trust policy


# SYNOPSIS
**podman image trust set|show**
[**-h**|**--help**]
[**-j**|**--json**]
[**--raw**]
[**-f**|**--pubkeysfile** KEY1 [**-f**|**--pubkeysfile** KEY2,...]]
[**-t**|**--type** signedBy|accept|reject]
REGISTRY[/REPOSITORY]

# DESCRIPTION
Manages which registries you trust as a source of container images based on its location.  The location is determined by the transport and the registry host of the image.  Using this container image `docker://docker.io/library/busybox` as an example, `docker` is the transport and `docker.io` is the registry host.

The trust policy describes a registry scope (registry and/or repository).  This trust can use public keys for signed images.

Trust is defined in **/etc/containers/policy.json** and is enforced when a user attempts to pull an image from a registry that is managed by policy.json.

The scope of the trust is evaluated from most specific to the least specific. In other words, a policy may be defined for an entire registry.  Or it could be defined for a particular repository in that registry. Or it could be defined down to a specific signed image inside of the registry.  See below for examples.

Trust **type** provides a way to:

Whitelist ("accept") or
Blacklist ("reject") registries.


Trust may be updated using the command **podman image trust set** for an existing trust scope.

# OPTIONS
**-h** **--help**
  Print usage statement.

**-f** **--pubkeysfile**
  A path to an exported public key on the local system. Key paths
  will be referenced in policy.json. Any path may be used but the path
  **/etc/pki/containers** is recommended. Options may be used multiple times to
  require an image be signed by multiple keys. One of **--pubkeys** or
  **--pubkeysfile** is required for the **signedBy** type.

**-t** **--type**
  The trust type for this policy entry. Accepted values:
    **signedBy** (default): Require signatures with corresponding list of
                        public keys
    **accept**: do not require any signatures for this
            registry scope
    **reject**: do not accept images for this registry scope

# show OPTIONS

**--raw**
  Output trust policy file as raw JSON

**-j** **--json**
  Output trust as JSON for machine parsing

# EXAMPLES

Accept all unsigned images from a registry

    sudo podman image trust set --type accept docker.io

Modify default trust policy

    sudo podman image trust set -t reject default

Display system trust policy

    sudo podman image trust show

Display trust policy file

   sudo podman image trust show --raw

Display trust as JSON

   sudo podman image trust show --json

# SEE ALSO

policy-json(5)

# HISTORY

January 2019, updated by Tom Sweeney (tsweeney at redhat dot com)

December 2018, originally compiled by Qi Wang (qiwan at redhat dot com)
