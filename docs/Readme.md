# Podman Documentation

The online man pages and other documents regarding Podman can be found at
[Read The Docs](https://podman.readthedocs.io).  The man pages
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

## API Reference

The [latest online documentation](http://docs.podman.io/en/latest/_static/api.html) is
automatically generated from committed upstream sources.  There is a short-duration
cache involved, in case old content or an error is returned, try clearing your browser
cache or returning to the site after 10-30 minutes.

***Maintainers Note***: Please refer to [the Cirrus-CI tasks
documentation](../contrib/cirrus/README.md#docs-task) for
important operational details.
