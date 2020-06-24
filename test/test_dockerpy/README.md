# Dockerpy regression test

Python test suite to validate Podman endpoints using dockerpy library

Validate :-

Make sure podman binary exist under libpod/bin

Validate Python 3.6+ installed using `python --version `

Validate docker-py module exist by using `python list`. If missing install.


Running tests
=============
To run the tests locally in your sandbox:

cd into the libpod folder and run one of the following commands:-

Note:- The test can be ran as root or rootless i.e sudo is optional.

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
