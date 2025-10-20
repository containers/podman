% podman-artifact-inspect 1

## NAME
podman\-artifact\-inspect - Inspect an OCI artifact

## SYNOPSIS
**podman artifact inspect** [*options*] *artifact*

## DESCRIPTION

This displays the low-level information on artifacts identified by name or digest. By default, this renders all results in a JSON array. If a format is specified, the given template is executed for each result.

The artifact can be referred to with either:

1. Fully qualified artifact name
2. Full or partial digest of the artifact's manifest

The inspect output includes the artifact manifest with annotations. All artifacts
automatically include a creation timestamp in the `org.opencontainers.image.created`
annotation using RFC3339Nano format, showing when the artifact was initially created.

## OPTIONS

#### **--format**, **-f**=*format*

Format the output using the given Go template.
The keys of the returned JSON can be used as the values for the --format flag (see examples below).

Valid placeholders for the Go template are listed below:

| **Placeholder**      | **Description**                                    |
| -------------------- | -------------------------------------------------- |
| .Digest              | Artifact digest (sha256:+64-char hash)             |
| .Manifest ...        | OCI manifest structure with layers and config      |
| .Manifest.Annotations | Annotations map in the manifest                   |
| .Manifest.ArtifactType | IANA media type of the artifact                  |
| .Manifest.Config     | Config descriptor                                  |
| .Manifest.Layers     | Array of layer descriptors                         |
| .Name                | Artifact name                                      |

#### **--help**

Print usage statement.

## EXAMPLES

Inspect an OCI artifact in the local store.
```
$ podman artifact inspect quay.io/ramalama/smollm2:latest
{
     "Manifest": {
          "schemaVersion": 2,
          "mediaType": "application/vnd.oci.image.manifest.v1+json",
          "artifactType": "application/vnd.cnai.model.manifest.v1+json",
          "config": {
               "mediaType": "application/vnd.oci.empty.v1+json",
               "digest": "sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a",
               "size": 2,
               "data": "e30="
          },
          "layers": [
               {
                    "mediaType": "application/octet-stream",
                    "digest": "sha256:4d2396b16114669389d7555c15a1592aad584750310f648edad5ca8c4eccda17",
                    "size": 1820414656,
                    "annotations": {
                         "file_name": "smollm2",
                         "name": "smollm2",
                         "org.opencontainers.image.title": "sha256-4d2396b16114669389d7555c15a1592aad584750310f648edad5ca8c4eccda17"
                    }
               },
               {
                    "mediaType": "text/plain; charset=utf-8",
                    "digest": "sha256:6c6b9193c4172ace7fee7ae8c669b581b15e7c0b676ba012915dfc3c58a0fe52",
                    "size": 559,
                    "annotations": {
                         "file_name": "config.json",
                         "name": "smollm2",
                         "org.opencontainers.image.title": "sha256-6c6b9193c4172ace7fee7ae8c669b581b15e7c0b676ba012915dfc3c58a0fe52"
                    }
               },
               {
                    "mediaType": "text/plain; charset=utf-8",
                    "digest": "sha256:dfebd0343bdd30da9bcbc152d57d9dad916eaecc9c63a09093fb45a1421fcbe6",
                    "size": 1834,
                    "annotations": {
                         "file_name": "chat_template",
                         "name": "smollm2",
                         "org.opencontainers.image.title": "sha256-dfebd0343bdd30da9bcbc152d57d9dad916eaecc9c63a09093fb45a1421fcbe6"
                    }
               },
               {
                    "mediaType": "text/plain; charset=utf-8",
                    "digest": "sha256:f99d59d28272408d477a36cf7786a0c68e425062aad209d75b3b2a68e533b5cc",
                    "size": 1910,
                    "annotations": {
                         "file_name": "chat_template_converted",
                         "name": "smollm2",
                         "org.opencontainers.image.title": "sha256-f99d59d28272408d477a36cf7786a0c68e425062aad209d75b3b2a68e533b5cc"
                    }
               }
          ],
          "annotations": {
               "file_name": "smollm2",
               "name": "smollm2",
               "org.opencontainers.image.created": "2025-10-17T13:49:26.309541703Z"
          }
     },
     "Name": "quay.io/ramalama/smollm2:latest",
     "Digest": "sha256:9bb205f978a995c7adceedb35d5e90ba00bffb37c55dadf3f84210f1c5cadcd6"
}
```

Inspect artifact name for the specified artifact:
```
$ podman artifact inspect --format '{{ .Name }}' myartifact
localhost/test/myartifact
```

Inspect artifact digest:
```
$ podman artifact inspect --format '{{ .Digest }}' myartifact
sha256:abc123def456...
```

Inspect artifact creation time from annotations:
```
$ podman artifact inspect --format '{{ index .Manifest.Annotations "org.opencontainers.image.created" }}' myartifact
2025-10-20T13:49:26.309541703Z
```

Inspect multiple artifacts:
```
$ podman artifact inspect artifact1 artifact2
[
    { ... artifact1 data ... },
    { ... artifact2 data ... }
]
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-artifact(1)](podman-artifact.1.md)**

## HISTORY
Sept 2024, Originally compiled by Brent Baude <bbaude@redhat.com>
