[![master](https://travis-ci.org/checkpoint-restore/go-criu.svg?branch=master)](https://travis-ci.org/checkpoint-restore/go-criu)

## go-criu -- Go bindings for [CRIU](https://criu.org/)

This repository provides Go bindings for CRIU. The code is based on the Go based PHaul
implementation from the CRIU repository. For easier inclusion into other Go projects the
CRIU Go bindings have been moved to this repository.

The Go bindings provide an easy way to use the CRIU RPC calls from Go without the need
to set up all the infrastructure to make the actual RPC connection to CRIU.

The following example would print the version of CRIU:
```
	c := criu.MakeCriu()
	version, err := c.GetCriuVersion()
	fmt.Println(version)
```
or to just check if at least a certain CRIU version is installed:
```
	c := criu.MakeCriu()
	result, err := c.IsCriuAtLeast(31100)
```

### License

The license of go-criu is the Apache 2.0 license.

