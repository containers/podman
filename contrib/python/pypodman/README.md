# pypodman - CLI for podman written in python

## Status: Active Development

See [libpod](https://github.com/containers/libpod/contrib/python/pypodman)

## Releases

To build the pypodman egg and install as user:

```sh
cd ~/libpod/contrib/python/pypodman
python3 setup.py clean -a && python3 setup.py sdist bdist
python3 setup.py install --user
```
Add `~/.local/bin` to your `PATH`  to run pypodman command.

## Running command:

### Against local podman service
```sh
$ pypodman images
```
### Against remote podman service
```sh
$ pypodman --host node001.example.org images
```
### Full help system available
```sh
$ pypodman -h
```
```sh
$ pypodman images -h
```
