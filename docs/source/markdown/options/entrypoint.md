####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--entrypoint**=*"command"* | *'["command", "arg1", ...]'*

Override the default ENTRYPOINT from the image.

The ENTRYPOINT of an image is similar to a COMMAND
because it specifies what executable to run when the container starts, but it is
(purposely) more difficult to override. The ENTRYPOINT gives a container its
default nature or behavior. When the ENTRYPOINT is set, the
container runs as if it were that binary, complete with default options. More options can be
passed in via the COMMAND. But, if a user wants to run
something else inside the container, the **--entrypoint** option allows a new
ENTRYPOINT to be specified.

Specify multi option commands in the form of a JSON string.
