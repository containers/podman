% podman-search "1"

## NAME
podman\-search - Search a registry for an image

## SYNOPSIS
**podman search** [*options*] *term*

## DESCRIPTION
**podman search** searches a registry or a list of registries for a matching image.
The user can specify which registry to search by prefixing the registry in the search term
(example **registry.fedoraproject.org/fedora**), default is the registries in the
**registires.search** table in the config file - **/etc/containers/registries.conf**.
The number of results can be limited using the **--limit** flag. If more than one registry
is being searched, the limit will be applied to each registry. The output can be filtered
using the **--filter** flag.

**podman [GLOBAL OPTIONS]**

**podman search [GLOBAL OPTIONS]**

**podman search [OPTIONS] TERM**

## OPTIONS

**--authfile**

Path of the authentication file. Default is ${XDG_\RUNTIME\_DIR}/containers/auth.json

**--filter, -f**

Filter output based on conditions provided (default [])

Supported filters are:

* stars (int - number of stars the image has)
* is-automated (boolean - true | false) - is the image automated or not
* is-official (boolean - true | false) - is the image official or not

**--format**

Change the output format to a Go template

Valid placeholders for the Go template are listed below:

| **Placeholder** | **Description**              |
| --------------- | ---------------------------- |
| .Index          | Registry                     |
| .Name           | Image name                   |
| .Descriptions   | Image description            |
| .Stars          | Star count of image          |
| .Official       | "[OK]" if image is official  |
| .Automated      | "[OK]" if image is automated |

**--limit**

Limit the number of results
Note: The results from each registry will be limited to this value.
Example if limit is 10 and two registries are being searched, the total
number of results will be 20, 10 from each (if there are at least 10 matches in each).
The order of the search results is the order in which the API endpoint returns the results.

**--no-trunc**

Do not truncate the output

**--tls-verify**

Require HTTPS and verify certificates when contacting registries (default: true). If explicitly set to true,
then tls verification will be used. If set to false then tls verification will not be used if needed. If not specified
default registries will be searched through (in /etc/containers/registries.conf), and tls will be skipped if a default
registry is listed in the insecure registries.

**--help**, **-h**

Print usage statement

## EXAMPLES

```
# podman search --limit 3 rhel
INDEX        NAME                                 DESCRIPTION                                       STARS   OFFICIAL   AUTOMATED
docker.io    docker.io/richxsl/rhel7              RHEL 7 image with minimal installation            9
docker.io    docker.io/bluedata/rhel7             RHEL-7.x base container images                    1
docker.io    docker.io/gidikern/rhel-oracle-jre   RHEL7 with jre8u60                                5                  [OK]
redhat.com   redhat.com/rhel                      This platform image provides a minimal runti...   0
redhat.com   redhat.com/rhel6                     This platform image provides a minimal runti...   0
redhat.com   redhat.com/rhel6.5                   This platform image provides a minimal runti...   0
```

```
# podman search alpine
INDEX       NAME                                             DESCRIPTION                                       STARS   OFFICIAL   AUTOMATED
docker.io   docker.io/library/alpine                         A minimal Docker image based on Alpine Linux...   3009    [OK]
docker.io   docker.io/mhart/alpine-node                      Minimal Node.js built on Alpine Linux             332
docker.io   docker.io/anapsix/alpine-java                    Oracle Java 8 (and 7) with GLIBC 2.23 over A...   272                [OK]
docker.io   docker.io/tenstartups/alpine                     Alpine linux base docker image with useful p...   5                  [OK]
```

```
# podman search registry.fedoraproject.org/fedora
INDEX               NAME                               DESCRIPTION   STARS   OFFICIAL   AUTOMATED
fedoraproject.org   fedoraproject.org/fedora                         0
fedoraproject.org   fedoraproject.org/fedora-minimal                 0
```

```
# podman search --filter=is-official alpine
INDEX       NAME                       DESCRIPTION                                       STARS   OFFICIAL   AUTOMATED
docker.io   docker.io/library/alpine   A minimal Docker image based on Alpine Linux...   3009    [OK]
```

```
# podman search --format "table {{.Index}} {{.Name}}" registry.fedoraproject.org/fedora
INDEX               NAME
fedoraproject.org   fedoraproject.org/fedora
fedoraproject.org   fedoraproject.org/fedora-minimal
```
## FILES

**registries.conf** (`/etc/containers/registries.conf`)

	registries.conf is the configuration file which specifies which container registries should be consulted when completing image names which do not include a registry or domain portion.

## SEE ALSO
podman(1), containers-registries.conf(5), crio(8)

## HISTORY
January 2018, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
