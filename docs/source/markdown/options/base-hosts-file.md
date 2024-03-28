####> This option file is used in:
####>   podman create, pod create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--base-hosts-file**=*path* | *image* | *none*

BaseHostsFile is the path to a hosts file, the entries from this file are added to the <<containers|pods>> _/etc/hosts_ file.
As special value "image" is allowed which uses the _/etc/hosts_ file from within the image and "none" which uses no base file at all.
If it is empty we should default to the base_hosts_file configuration in _containers.conf_.
