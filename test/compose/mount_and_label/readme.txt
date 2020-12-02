this test creates a container with a mount (not volume) and also adds a label to the container.

validate by curl http://localhost:5000 and message should be same message as piped into the mount message.

also verify the label with podman ps and a filter that only catches that container
