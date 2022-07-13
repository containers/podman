#### **--shm-size**=*size*

Size of `/dev/shm`. The format is `<number><unit>`.
If the unit is omitted, the system uses bytes. If the size is omitted, the system uses `64m`.
When size is `0`, there is no limit on the amount of memory used for IPC by the <POD-OR-CONTAINER>. This option conflicts with **--ipc=host** when running containers.
