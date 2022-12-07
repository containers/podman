####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--name**=*name*

Assign a name to the container.

The operator can identify a container in three ways:

- UUID long identifier (“f78375b1c487e03c9438c729345e54db9d20cfa2ac1fc3494b6eb60872e74778”);
- UUID short identifier (“f78375b1c487”);
- Name (“jonah”).

Podman generates a UUID for each container, and if a name is not assigned
to the container with **--name** then it will generate a random
string name. The name can be useful as a more human-friendly way to identify containers.
This works for both background and foreground containers.
