% podman-image-trust "1"

# NAME
podman\-trust - Manage container image trust policy


# SYNOPSIS
**podman image trust set|show**
[**-h**|**--help**]
[**-j**|**--json**]
[**--raw**]
[**-f**|**--pubkeysfile** KEY1 [**f**|**--pubkeysfile** KEY2,...]]
[**-t**|**--type** signedBy|accept|reject]
REGISTRY[/REPOSITORY]

# DESCRIPTION
Manages the trust policy of the host system. Trust policy describes
a registry scope (registry and/or repository) that must be signed by public keys. Trust
is defined in **/etc/containers/policy.json**. Trust is enforced when a user attempts to pull
an image from a registry.

Trust scope is evaluated by most specific to least specific. In other words, policy may
be defined for an entire registry, but refined for a particular repository in that
registry. See below for examples.

Trust **type** provides a way to whitelist ("accept") or blacklist
("reject") registries.

Trust may be updated using the command **podman image trust set** for an existing trust scope.

# OPTIONS
**-h** **--help**
  Print usage statement.

**-f** **--pubkeysfile**
  A path to an exported public key on the local system. Key paths
  will be referenced in policy.json. Any path may be used but path
  **/etc/pki/containers** is recommended. Option may be used multiple times to
  require an image be sigend by multiple keys. One of **--pubkeys** or
  **--pubkeysfile** is required for **signedBy** type.

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

    podman image trust set --type accept docker.io

Modify default trust policy

    podman image trust set -t reject default

Display system trust policy

    podman image trust show

Display trust policy file

    podman image trust show --raw

Display trust as JSON

    podman image trust show --json

# HISTORY
December 2018, originally compiled by Qi Wang (qiwan at redhat dot com)
