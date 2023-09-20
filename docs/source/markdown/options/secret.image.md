####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--secret**=**id=id,src=path**

Pass secret information used in the Containerfile for building images
in a safe way that are not stored in the final image, or be seen in other stages.
The secret is mounted in the container at the default location of `/run/secrets/id`.

To later use the secret, use the --mount option in a `RUN` instruction within a `Containerfile`:

`RUN --mount=type=secret,id=mysecret cat /run/secrets/mysecret`
