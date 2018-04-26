% podman(1) podman-search - Tool to search registries for an image
% Urvashi Mohnani
# podman-search "1" "January 2018" "podman"

## NAME
podman\-search - Search a registry for an image

## SYNOPSIS
**podman search**
**TERM**
[**--filter**|**-f**]
[**--format**]
[**--limit**]
[**--no-trunc**]
[**--registry**]
[**--help**|**-h**]

## DESCRIPTION
**podman search** searches a registry or a list of registries for a matching image.
The user can specify which registry to search by setting the **--registry** flag, default
is the default registries set in the config file - **/etc/containers/registries.conf**.
The number of results can be limited using the **--limit** flag. If more than one registry
is being searched, the limit will be applied to each registry. The output can be filtered
using the **--filter** flag.

**podman [GLOBAL OPTIONS]**

**podman search [GLOBAL OPTIONS]**

**podman search [OPTIONS] TERM**

## OPTIONS

**--filter, -f**
Filter output based on conditions provided (default [])

Supported filters are:
- stars (int - number of stars the image has)
- is-automated (boolean - true | false) - is the image automated or not
- is-official (boolean - true | false) - is the image official or not

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

**--registry**
Specific registry to search (only the given registry will be searched, not the default registries)

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
# podman search --registry registry.fedoraproject.org fedora
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
# podman search --registry registry.fedoraproject.org --format "table {{.Index}} {{.Name}}" fedora
INDEX               NAME
fedoraproject.org   fedoraproject.org/fedora
fedoraproject.org   fedoraproject.org/fedora-minimal
```

## SEE ALSO
podman(1), crio(8)

## HISTORY
January 2018, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
