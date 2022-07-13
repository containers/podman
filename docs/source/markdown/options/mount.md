#### **--mount**=*type=TYPE,TYPE-SPECIFIC-OPTION[,...]*

Attach a filesystem mount to the container

Current supported mount TYPEs are **bind**, **volume**, **image**, **tmpfs** and **devpts**. <sup>[[1]](#Footnote1)</sup>

       e.g.

       type=bind,source=/path/on/host,destination=/path/in/container

       type=bind,src=/path/on/host,dst=/path/in/container,relabel=shared

       type=bind,src=/path/on/host,dst=/path/in/container,relabel=shared,U=true

       type=volume,source=vol1,destination=/path/in/container,ro=true

       type=tmpfs,tmpfs-size=512M,destination=/path/in/container

       type=image,source=fedora,destination=/fedora-image,rw=true

       type=devpts,destination=/dev/pts

       Common Options:

	      · src, source: mount source spec for bind and volume. Mandatory for bind.

	      · dst, destination, target: mount destination spec.

       Options specific to volume:

	      · ro, readonly: true or false (default).

	      . U, chown: true or false (default). Change recursively the owner and group of the source volume based on the UID and GID of the container.

	      · idmap: true or false (default).  If specified, create an idmapped mount to the target user namespace in the container.

       Options specific to image:

	      · rw, readwrite: true or false (default).

       Options specific to bind:

	      · ro, readonly: true or false (default).

	      · bind-propagation: shared, slave, private, unbindable, rshared, rslave, runbindable, or rprivate(default). See also mount(2).

	      . bind-nonrecursive: do not set up a recursive bind mount. By default it is recursive.

	      . relabel: shared, private.

	      · idmap: true or false (default).  If specified, create an idmapped mount to the target user namespace in the container.

	      . U, chown: true or false (default). Change recursively the owner and group of the source volume based on the UID and GID of the container.

       Options specific to tmpfs:

	      · ro, readonly: true or false (default).

	      · tmpfs-size: Size of the tmpfs mount in bytes. Unlimited by default in Linux.

	      · tmpfs-mode: File mode of the tmpfs in octal. (e.g. 700 or 0700.) Defaults to 1777 in Linux.

	      · tmpcopyup: Enable copyup from the image directory at the same location to the tmpfs. Used by default.

	      · notmpcopyup: Disable copying files from the image to the tmpfs.

	      . U, chown: true or false (default). Change recursively the owner and group of the source volume based on the UID and GID of the container.

       Options specific to devpts:

	      · uid: UID of the file owner (default 0).

	      · gid: GID of the file owner (default 0).

	      · mode: permission mask for the file (default 600).

	      · max: maximum number of PTYs (default 1048576).
