####> This option file is used in:
####>   podman artifact pull, artifact push, build, create, farm build, pull, push, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--retry-delay**=*duration*

Duration of delay between retry attempts when pulling or pushing images between
the registry and local storage in case of failure. The default is to start at two seconds and then exponentially back off. The delay is used when this value is set, and no exponential back off occurs.
