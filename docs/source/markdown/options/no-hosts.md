####> This option file is used in:
####>   podman build, create, kube play, pod create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--no-hosts**

Do not create _/etc/hosts_ for the <<container|pod>>.
By default, Podman will manage _/etc/hosts_, adding the container's own IP address and any hosts from **--add-host**.
**--no-hosts** disables this, and the image's _/etc/hosts_ will be preserved unmodified.
