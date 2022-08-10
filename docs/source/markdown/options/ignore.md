#### **--ignore**, **-i**

Ignore errors when specified <<containers|pods>> are not in the container store.  A user
might have decided to manually remove a <<container|pod>> which would lead to a failure
during the ExecStop directive of a systemd service referencing that <<container|pod>>.
