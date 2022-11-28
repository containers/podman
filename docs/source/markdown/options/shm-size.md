####> This option file is used in:
####>   podman build, create, pod clone, pod create, run
####> If you edit this file, make sure your changes
####> are applicable to all of those files.
#### **--shm-size**=*number[unit]*

Size of _/dev/shm_. A _unit_ can be **b** (bytes), **k** (kibibytes), **m** (mebibytes), or **g** (gibibytes).
If the unit is omitted, the system uses bytes. If the size is omitted, the default is **64m**.
When _size_ is **0**, there is no limit on the amount of memory used for IPC by the <<container|pod>>.
This option conflicts with **--ipc=host**.
