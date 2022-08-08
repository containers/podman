#### **--read-only**

Mount the container's root filesystem as read-only.

By default a container will have its root filesystem writable allowing processes
to write files anywhere. By specifying the **--read-only** flag, the container will have
its root filesystem mounted as read-only prohibiting any writes.
