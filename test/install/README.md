# Installation Tests

The Dockerfiles in this directory attempt to install the RPMs built from this
repo into the target OS. Make the RPMs first with:

```
make -f .copr/Makefile srpm outdir=test/install/rpms
make -f .copr/Makefile build_binary outdir=test/install/rpms
```

Then, run a container image build using the Dockerfiles in this directory.
