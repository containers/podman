# Dockerpy regression test

Python test suite to validate Podman endpoints using dockerpy library

## Running Tests

To run the tests locally in your sandbox (Fedora 32):

```shell script
# dnf install python3-docker
```

### Run the entire test suite

```shell
# cd test/python/dockerpy
# PYTHONPATH=/usr/bin/python python -m unittest discover .
```

Passing the -v option to your test script will instruct unittest.main() to enable a higher level of verbosity, and produce detailed output:

```shell
# cd test/python/dockerpy
# PYTHONPATH=/usr/bin/python python -m unittest -v discover .
```

### Run a specific test class

```shell
# cd test/python/dockerpy
# PYTHONPATH=/usr/bin/python python -m unittest -v tests.test_images
```

### Run a specific test within the test class

```shell
# cd test/python/dockerpy
# PYTHONPATH=/usr/bin/python python -m unittest tests.test_images.TestImages.test_import_image

```
