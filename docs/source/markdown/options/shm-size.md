#### **--shm-size**=*number[unit]*

Size of _/dev/shm_. A _unit_ can be **b** (bytes), **k** (kibibytes), **m** (mebibytes), or **g** (gibibytes).
If you omit the unit, the system uses bytes. If you omit the size entirely, the default is **64m**.
When _size_ is **0**, there is no limit on the amount of memory used for IPC by the <<container|pod>>.
This option conflicts with **--ipc=host**.
