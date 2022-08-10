#### **--http-proxy**

By default proxy environment variables are passed into the container if set
for the Podman process. This can be disabled by setting the value to **false**.
The environment variables passed in include **http_proxy**,
**https_proxy**, **ftp_proxy**, **no_proxy**, and also the upper case versions of
those. This option is only needed when the host system must use a proxy but
the container should not use any proxy. Proxy environment variables specified
for the container in any other way will override the values that would have
been passed through from the host. (Other ways to specify the proxy for the
container include passing the values with the **--env** flag, or hard coding the
proxy environment at container build time.)
(This option is not available with the remote Podman client, including Mac and Windows (excluding WSL2) machines)

Defaults to **true**.
