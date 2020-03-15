# Podman Documentation

The online man pages and other documents regarding Podman can be found at
[Read The Docs](https://podman.readthedocs.io/en/latest/index.html).  The man pages
can be found under the [Commands](https://podman.readthedocs.io/en/latest/Commands.html)
link on that page.

# Build the Docs

## Directory Structure

|                                      | Directory                   |
| ------------------------------------ | --------------------------- |
| Markdown source for man pages        | docs/source/markdown/       |
| man pages aliases as .so files       | docs/source/markdown/links/ |
| restructured text for readthedocs.io | docs/rst/                   |
| target for output                    | docs/build                  |
| man pages                            | docs/build/man              |
| remote linux man pages               | docs/build/remote/linux     |
| remote darwin man pages              | docs/build/remote/darwin    |
| remote windows html pages            | docs/build/remote/windows   |

## Support files

| | |
| ------------------------------------ | --------------------------- |
| docs/remote-docs.sh | Read the docs/source/markdown files and format for each platform |
| docs/links-to-html.lua | pandoc filter to do aliases for html files |
