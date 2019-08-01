![PODMAN logo](../../logo/podman-logo-source.svg)

A container image for canonical-naming and uploading of
libpod and remote-client archives.  Only intended to ever
be used by CI/CD, and depends heavily on an embedded
`release.txt` file produced by `make`.

Build  script: [../cirrus/build_release.sh](../cirrus/build_release.sh)
Upload script: [../cirrus/upload_release_archive.sh](../cirrus/upload_release_archive.sh)
