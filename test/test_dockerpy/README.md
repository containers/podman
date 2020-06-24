# Dockerpy regression test

Python test suite to validate Podman endpoints using dockerpy library

Validate :-

Make sure to build the `podman` binary via `make podman`.

Validate Python 3.6+ is installed using `python --version`.

Validate the `docker-py` Python module exist by using `python list`.


Running tests
=============
Note:- The test can be ran as root (e.g., via `sudo`) or as an ordinary rootless user.

#### Run the entire test suite

```
sudo PYTHONPATH=/usr/bin/python python -m unittest discover test/
```

Passing the -v option to your test script will instruct unittest.main() to enable a higher level of verbosity, and produce detailed output:

```
sudo PYTHONPATH=/usr/bin/python python -m unittest discover -v test/
```

#### Run a specific test class

```
sudo PYTHONPATH=/usr/bin/python python -m unittest -v test.test_dockerpy.test_images
```

#### Run a specific test within the test class

```
sudo PYTHONPATH=/usr/bin/python python -m unittest -v test.test_dockerpy.test_images.TestImages.test_list_images
```
