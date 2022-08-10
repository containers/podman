#### **--no-hosts**

Do not create _/etc/hosts_ for the <<container|pod>>.
By default, Podman will manage _/etc/hosts_, adding the container's own IP address and any hosts from **--add-host**.
**--no-hosts** disables this, and the image's _/etc/hosts_ will be preserved unmodified.
