# Dockerpy regression test

Python test suite to validate Podman endpoints using dockerpy library

Running tests
=============
To run the tests locally in your sandbox:

#### Make sure that the Podman system service is running to do so

```
sudo podman --log-level=debug system service -t0 unix:/run/podman/podman.sock
```
#### Run the entire test

```
sudo PYTHONPATH=/usr/bin/python python -m dockerpy.images
```

Passing the -v option to your test script will instruct unittest.main() to enable a higher level of verbosity, and produce detailed output:

```
sudo PYTHONPATH=/usr/bin/python python -m unittest -v dockerpy.images
```
#### Run a specific test class

```
sudo PYTHONPATH=/usr/bin/python python -m unittest -v dockerpy.images.TestImages
```

#### Run a specific test within the test class

```
sudo PYTHONPATH=/usr/bin/python python -m unittest -v dockerpy.images.TestImages.test_list_images
```
