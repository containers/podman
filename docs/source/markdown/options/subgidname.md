####> This option file is used in:
####>   podman create, pod clone, pod create, run
####> If you edit this file, make sure your changes
####> are applicable to all of those files.
#### **--subgidname**=*name*

Run the container in a new user namespace using the map with _name_ in the _/etc/subgid_ file.
If running rootless, the user needs to have the right to use the mapping. See **subgid**(5).
This flag conflicts with **--userns** and **--gidmap**.
