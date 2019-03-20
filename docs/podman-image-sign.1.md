% podman-image-sign(1)

# NAME
podman-image-sign - Create a signature for an image

# SYNOPSIS
**podman image sign**
[**--help**|**-h**]
[**--directory**|**-d**]
[**--sign-by**]
[ IMAGE... ]

# DESCRIPTION
**podmain image sign** will create a local signature for one or more local images that have
been pulled from a registry. The signature will be written to a directory
derived from the registry configuration files in /etc/containers/registries.d. By default, the signature will be written into /var/lib/containers/sigstore directory.

# OPTIONS
**--help** **-h**
  Print usage statement.

**--directory** **-d**
  Store the signatures in the specified directory.  Default: /var/lib/containers/sigstore

**--sign-by**
  Override the default identity of the signature.

# EXAMPLES
Sign the busybox image with the identify of foo@bar.com with a user's keyring and save the signature in /tmp/signatures/.

   sudo podman image sign --sign-by foo@bar.com --directory /tmp/signatures docker://privateregistry.example.com/foobar

# RELATED CONFIGURATION

The write (and read) location for signatures is defined in YAML-based
configuration files in /etc/containers/registries.d/.  When you sign
an image, podman will use those configuration files to determine
where to write the signature based on the the name of the originating
registry or a default storage value unless overriden with the --directory
option. For example, consider the following configuration file.

docker:
  privateregistry.example.com:
    sigstore: file:///var/lib/containers/sigstore

When signing an image preceeded with the registry name 'privateregistry.example.com',
the signature will be written into subdirectories of
/var/lib/containers/sigstore/privateregistry.example.com. The use of 'sigstore' also means
the signature will be 'read' from that same location on a pull-related function.

# HISTORY
November 2018, Originally compiled by Qi Wang (qiwan at redhat dot com)
