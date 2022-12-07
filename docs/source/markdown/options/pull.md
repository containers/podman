####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--pull**=*policy*

Pull image policy. The default is **missing**.

- **always**: Always pull the image and throw an error if the pull fails.
- **missing**: Pull the image only if it could not be found in the local containers storage.  Throw an error if no image could be found and the pull fails.
- **never**: Never pull the image but use the one from the local containers storage.  Throw an error if no image could be found.
- **newer**: Pull if the image on the registry is newer than the one in the local containers storage.  An image is considered to be newer when the digests are different.  Comparing the time stamps is prone to errors.  Pull errors are suppressed if a local image was found.
