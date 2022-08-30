#### **--subuidname**=*name*

Run the container in a new user namespace using the map with _name_ in the _/etc/subuid_ file.
If running rootless, the user needs to have the right to use the mapping. See **subuid**(5).
This flag conflicts with **--userns** and **--uidmap**.
