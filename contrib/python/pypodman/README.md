# pypodman - CLI interface for podman written in python

## Status: Active Development

See [libpod](https://github.com/projectatomic/libpod/contrib/python/cmd)

## Releases

To build the pypodman egg:

```sh
cd ~/libpod/contrib/python/cmd
python3 setup.py clean -a && python3 setup.py bdist
```

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
