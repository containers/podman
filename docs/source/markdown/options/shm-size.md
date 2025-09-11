####> This option file is used in:
####>   podman build, create, farm build, pod clone, pod create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--shm-size**=*number[unit]*

Size of _/dev/shm_. A _unit_ can be **b** (bytes), **k** (kibibytes), **m** (mebibytes), or **g** (gibibytes).
If the unit is omitted, the system uses bytes. If the size is omitted, the default is **64m**.
When _size_ is **0**, there is no limit on the amount of memory used for IPC by the <<container|pod>>.
This option conflicts with **--ipc=host**.
