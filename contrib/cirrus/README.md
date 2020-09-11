![PODMAN logo](../../logo/podman-logo-source.svg)

# Cirrus-CI

Similar to other integrated github CI/CD services, Cirrus utilizes a simple
YAML-based configuration/description file: ``.cirrus.yml``.  Ref: https://cirrus-ci.org/


## Workflow

All tasks execute in parallel, unless there are conditions or dependencies
which alter this behavior.  Within each task, each script executes in sequence,
so long as any previous script exited successfully.  The overall state of each
task (pass or fail) is set based on the exit status of the last script to execute.

### ``gating`` Task

***N/B: Steps below are performed by automation***

1. Launch a purpose-built container in Cirrus's community cluster.
   For container image details, please see
   [the contributors guide](https://github.com/containers/podman/blob/master/CONTRIBUTING.md#go-format-and-lint).

3. ``validate``: Perform standard `make validate` source verification,
   Should run for less than a minute or two.

4. ``lint``: Execute regular `make lint` to check for any code cruft.
   Should also run for less than a few minutes.

5. ``vendor``: runs `make vendor-in-container` followed by `./hack/tree_status.sh` to check
   whether the git tree is clean. The reasoning for that is to make sure that
   the vendor.conf, the code and the vendored packages in ./vendor are in sync
   at all times.

### ``meta`` Task

***N/B: Steps below are performed by automation***

1. Launch a container built from definition in ``./contrib/imgts``.

2. Update VM Image metadata to help track usage across all automation.

4. Always exits successfully unless there's a major problem.


### ``testing`` Task

***N/B: Steps below are performed by automation***

1. After `gating` passes, spin up one VM per
   `matrix: image_name` item. Once accessible, ``ssh``
   into each VM as the `root` user.

2. ``setup_environment.sh``: Configure root's `.bash_profile`
    for all subsequent scripts (each run in a new shell).  Any
    distribution-specific environment variables are also defined
    here.  For example, setting tags/flags to use compiling.

5. ``integration_test.sh``: Execute integration-testing.  This is
   much more involved, and relies on access to external
   resources like container images and code from other repositories.
   Total execution time is capped at 2-hours (includes all the above)
   but this script normally completes in less than an hour.


### ``special_testing_cross`` Task

Confirm that cross-compile of podman-remote functions for both `windows`
and `darwin` targets.


### ``special_testing_cgroupv2`` Task

Use the latest Fedora release with the required kernel options pre-set for
exercising cgroups v2 with Podman integration tests.  Also depends on
having `SPECIALMODE` set to 'cgroupv2`


### `docs` Task

Builds swagger API documentation YAML and uploads to google storage (an online
service for storing unstructured data) for both
PR's (for testing the process) and the master branch.  For PR's
the YAML is uploaded into a [dedicated short-pruning cycle
bucket.](https://storage.googleapis.com/libpod-pr-releases/) for testing purposes
only.  For the master branch, a [separate bucket is
used](https://storage.googleapis.com/libpod-master-releases) and provides the
content rendered on [the API Reference page](https://docs.podman.io/en/latest/_static/api.html)

The online API reference is presented by javascript to the client.  To prevent hijacking
of the client by malicious data, the [javascript utilises CORS](https://cloud.google.com/storage/docs/cross-origin).
This CORS metadata is served by `https://storage.googleapis.com` when configured correctly.
It will appear in [the request and response headers from the
client](https://cloud.google.com/storage/docs/configuring-cors#troubleshooting) when accessing
the API reference page.

However, when the CORS metadata is missing or incorrectly configured, clients will receive an
error-message similar to:

![Javascript Stack Trace Image](swagger_stack_trace.png)

For documentation built by Read The Docs from the master branch, CORS metadata is
set on the `libpod-master-releases` storage bucket.  Viewing or setting the CORS
metadata on the bucket requires having locally [installed and
configured the google-cloud SDK](https://cloud.google.com/sdk/docs).  It also requires having
admin access to the google-storage bucket.  Contact a project owner for help if you are
unsure of your permissions or need help resolving an error similar to the picture above.

Assuming the SDK is installed, and you have the required admin access, the following command
will display the current CORS metadata:

```
gsutil cors get gs://libpod-master-releases
```

To function properly (allow client "trust" of content from `storage.googleapis.com`) the followiing
metadata JSON should be used.  Following the JSON, is an example of the command used to set this
metadata on the libpod-master-releases bucket.  For additional information about configuring CORS
please refer to [the google-storage documentation](https://cloud.google.com/storage/docs/configuring-cors).

```JSON
[
    {
      "origin": ["http://docs.podman.io", "https://docs.podman.io"],
      "responseHeader": ["Content-Type"],
      "method": ["GET"],
      "maxAgeSeconds": 600
    }
]
```

```
gsutil cors set /path/to/file.json gs://libpod-master-releases
```

***Note:*** The CORS metadata does _NOT_ change after the `docs` task uploads a new swagger YAML
file.  Therefore, if it is not functioning or misconfigured, a person must have altered it or
changes were made to the referring site (e.g. `docs.podman.io`).

## `$SPECIALMODE`

Some tasks alter their behavior based on this value.  A summary of supported
values follows:

* `none`: Operate as normal, this is the default value if unspecified.
* `rootless`: Causes a random, ordinary user account to be created
              and utilized for testing.
* `in_podman`: Causes testing to occur within a container executed by
* `windows`: See **darwin**
* `darwin`: Signals the ``special_testing_cross`` task to cross-compile the remote client.
