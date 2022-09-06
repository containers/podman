% podman-kube-down 1

## NAME
podman-kube-down - Remove containers and pods based on Kubernetes YAML

## SYNOPSIS
**podman kube down** *file.yml|-|https://website.io/file.yml*

## DESCRIPTION
**podman kube down** reads a specified Kubernetes YAML file, tearing down pods that were created by the `podman kube play` command via the same Kubernetes YAML
file. Any volumes that were created by the previous `podman kube play` command remain intact. If the YAML file is specified as `-`, `podman kube down` reads the
YAML from stdin. The input can also be a URL that points to a YAML file such as https://podman.io/demo.yml. `podman kube down` will then teardown the pods and
containers created by `podman kube play` via the same Kubernetes YAML from the URL. However, `podman kube down` will not work with a URL if the YAML file the URL
points to has been changed or altered since the creation of the pods and containers using `podman kube play`.

## EXAMPLES

Example YAML file `demo.yml`:
```
apiVersion: v1
kind: Pod
metadata:
...
spec:
  containers:
  - command:
    - top
    - name: container
      value: podman
    image: foobar
...
```

Remove the pod and containers as described in the `demo.yml` file
```
$ podman kube down demo.yml
Pods stopped:
52182811df2b1e73f36476003a66ec872101ea59034ac0d4d3a7b40903b955a6
Pods removed:
52182811df2b1e73f36476003a66ec872101ea59034ac0d4d3a7b40903b955a6
```

Remove the pod and containers as described in the `demo.yml` file YAML sent to stdin
```
$ cat demo.yml | podman kube play -
Pods stopped:
52182811df2b1e73f36476003a66ec872101ea59034ac0d4d3a7b40903b955a6
Pods removed:
52182811df2b1e73f36476003a66ec872101ea59034ac0d4d3a7b40903b955a6
```

Remove the pods and containers as described in the `demo.yml` file YAML read from a URL
```
$ podman kube down https://podman.io/demo.yml
Pods stopped:
52182811df2b1e73f36476003a66ec872101ea59034ac0d4d3a7b40903b955a6
Pods removed:
52182811df2b1e73f36476003a66ec872101ea59034ac0d4d3a7b40903b955a6
```
`podman kube down` will not work with a URL if the YAML file the URL points to has been changed
or altered since it was used to create the pods and containers.

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-kube(1)](podman-kube.1.md)**, **[podman-kube-play(1)](podman-kube-play.1.md)**, **[podman-kube-generate(1)](podman-kube-generate.1.md)**, **[containers-certs.d(5)](https://github.com/containers/image/blob/main/docs/containers-certs.d.5.md)**
