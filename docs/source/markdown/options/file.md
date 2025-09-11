####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--file**, **-f**=*Containerfile*

Specifies a Containerfile which contains instructions for building the image,
either a local file or an **http** or **https** URL.  If more than one
Containerfile is specified, *FROM* instructions are only be accepted from the
last specified file.

If a build context is not specified, and at least one Containerfile is a
local file, the directory in which it resides is used as the build
context.

Specifying the option `-f -` causes the Containerfile contents to be read from stdin.
