% podman-patch(1)

## NAME
podman\-patch - Patch file inside existing running containers

## SYNOPSIS
**podman patch** [*options*] *file-to-patch* *source-file-patch* *[containers]*

## DESCRIPTION
`podman patch` patch an existing running container file. If user pass
the flag `--all` all the running containers are patched.
If the file to patch doesn't exist inside the running container 
this command exit in error. If you want to ignore case
where the running container doesn't have the file to patch the `--ignore-fail` flag can be pass
and then the command continue to run on other containers.

## OPTIONS

**--all, -a**

Apply the patch to all the running containers

**--ignore-fail, -i**

If the file to patch doesn't exist on a running container and this
flag was passed by user then `podman patch` skip to apply the patch and 
continue on other containers.

## EXAMPLES

```
$ podman run -it --name cont1 ubuntu bash &
$ podman run -it --name cont2 ubuntu bash &
$ diff /etc/profile ~/profile > /tmp/profile.patch
$ podman patch /etc/profile /tmp/profile.patch --all
Patching:  cont2
Execute: patch  /var/lib/containers/storage/overlay/1effd969dde77fc5ba09350bb7e1e3a56e1cf68e4a1633cf6320f4dbfeec14a6/merged/etc/profile /tmp/profile.patch
cont2 patched successfully!
Patching:  cont1
Execute: patch  /var/lib/containers/storage/overlay/fa72f57736ec30550a1bcff15d8246c2109ebb781f006771dab963d739294d77/merged/etc/profile /tmp/profile.patch
cont1 patched successfully!

```
$ podman patch /etc/profile /tmp/profile.patch cont1
Patching:  cont1
Execute: patch  /var/lib/containers/storage/overlay/fa72f57736ec30550a1bcff15d8246c2109ebb781f006771dab963d739294d77/merged/etc/profile /tmp/profile.patch
cont1 patched successfully!
```

```
$ podman patch /etc/boum /tmp/profile.patch cont1
Patching:  cont1
File to patch not found: /etc/boum
```

## SEE ALSO
podman(1), podman-commit(1)

## HISTORY
February 2019, Originally compiled by Hervé Beraud <hberaud@redhat.com>
