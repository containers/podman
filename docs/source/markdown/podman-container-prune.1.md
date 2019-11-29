% podman-container-prune(1)

## NAME
podman-container-prune - Remove all stopped containers from local storage

## SYNOPSIS
**podman container prune** [*options*]

## DESCRIPTION
**podman container prune** removes all stopped containers from local storage.

## OPTIONS

**-h**, **--help**

Print usage statement

## Examples

Remove all stopped containers from local storage
```
$ sudo podman container prune
WARNING! This will remove all stopped containers.
Are you sure you want to continue? [y/N] y
878392adf2e6c5c9bb1fc19b69d37d2e98c8abf9d539c0bce4b15b46bbcce471
37664467fbe3618bf9479c34393ac29c02696675addf1750f9e346581636cde7
ed0c6468b8e1cb641b4621d1fe30cb477e1fefc5c0bceb66feaf2f7cb50e5962
6ac6c8f0067b7a4682e6b8e18902665b57d1a0e07e885d9abcd382232a543ccd
fff1c5b6c3631746055ec40598ce8ecaa4b82aef122f9e3a85b03b55c0d06c23
602d343cd47e7cb3dfc808282a9900a3e4555747787ec6723bb68cedab8384d5
```

Remove all stopped containers from local storage without confirmation.
```
$ sudo podman container prune -f
878392adf2e6c5c9bb1fc19b69d37d2e98c8abf9d539c0bce4b15b46bbcce471
37664467fbe3618bf9479c34393ac29c02696675addf1750f9e346581636cde7
ed0c6468b8e1cb641b4621d1fe30cb477e1fefc5c0bceb66feaf2f7cb50e5962
6ac6c8f0067b7a4682e6b8e18902665b57d1a0e07e885d9abcd382232a543ccd
fff1c5b6c3631746055ec40598ce8ecaa4b82aef122f9e3a85b03b55c0d06c23
602d343cd47e7cb3dfc808282a9900a3e4555747787ec6723bb68cedab8384d5

```

Remove all stopped containers from local storage created within last 10 minutes
```
$ sudo podman container prune --filter until="10m"
WARNING! This will remove all stopped containers.
Are you sure you want to continue? [y/N] y
3d366295e33d8cc612c4d873199bacadd55088d90d17dcafaa9a2d317ad50b4e
```

## SEE ALSO
podman(1), podman-ps

## HISTORY
December 2018, Originally compiled by Brent Baude (bbaude at redhat dot com)
