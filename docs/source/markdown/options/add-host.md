####> This option file is used in:
####>   podman build, create, farm build, pod create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--add-host**=*host:ip*

Add a custom host-to-IP mapping (host:ip)
Multiple hostnames for the same IP can be separated by semicolons.

Add a line to /etc/hosts. The format is hostname:ip or hostname1;hostname2;hostname3:ip if you want to map multiple hostnames to the same ip without duplicating the --add-host parameter. The **--add-host**
option can be set multiple times. Conflicts with the **--no-hosts** option.
