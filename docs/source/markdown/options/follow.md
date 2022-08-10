#### **--follow**, **-f**

Follow log output.  Default is false.

Note: If you are following a <<container|pod>> which is removed by `podman <<container|pod>> rm`
or removed on exit (`podman run --rm ...`), then there is a chance that the log
file will be removed before `podman<< pod|>> logs` reads the final content.
