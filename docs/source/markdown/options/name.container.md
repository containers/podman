#### **--name**=*name*

Assign a name to the container.

The operator can identify a container in three ways:

- UUID long identifier (“f78375b1c487e03c9438c729345e54db9d20cfa2ac1fc3494b6eb60872e74778”);
- UUID short identifier (“f78375b1c487”);
- Name (“jonah”).

Podman generates a UUID for each container, and if a name is not assigned
to the container with **--name** then it will generate a random
string name. The name is useful any place you need to identify a container.
This works for both background and foreground containers.
