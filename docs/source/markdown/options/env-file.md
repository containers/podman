####> This option file is used in:
####>   podman create, exec, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--env-file**=*file*

Read the environment variables from the file, supporting prefix matching: `KEY*`, as well as multiline values in double quotes and single quotes, but not multiline values in backticks.
The env-file will ignore comments and empty lines. And spaces or tabs before and after the KEY.
If an invalid value is encountered, such as only an `=` sign, it will be skipped. If it is a prefix match (`KEY*`), all environment variables starting with KEY on the host machine will be loaded.
If it is only KEY (`KEY`), the KEY environment variable on the host machine will be loaded.
Compatible with the `export` syntax in **dotenv**, such as: `export KEY=bar`.
