# Change Request

## **Short Summary**

Implement the functionality to mount host certificates from the host system to
the Podman Machine OS.

## **Objective**

Users want the workloads running in containers to trust the Certificate
Authorities (CA) certificates that are installed in the host OS keystore.

The procedure to add the locally trusted certificates in the Podman machine is
currently documented as a manual procedure in both:

- [Podman tutorials](https://github.com/containers/podman/blob/main/docs/tutorials/podman-install-certificate-authority.md)
- [Podman Desktop Guide](https://podman-desktop.io/docs/podman/adding-certificates-to-a-podman-machine)

The objective is to automate this procedure and import the native trusted CA
certificates into the Podman machine's guest OS.

The certificates are imported during the start of a Podman machine.

When the Podman machine is running, restarting is required to synchronize
the list of trusted certificates.

## **Detailed Description:**

### Introducing a new `machine start` flag

We are going to introduce a new `pomdan machine start` flag
named `--import-native-ca`. To import the certificates a user needs to specify
`--import-native-ca` when starting the machine (using `podman machine start`).
The parameter's default value is `false`.

### Fetching the locally trusted certificates

To import the locally trusted certificates, the Podman client will retrieve the
trusted certificates from the local certificates stores.

The location of these stores may vary depending on the host OS, but Go provides
some abstractions to access the stores without worrying about the underlying
details.

For instance package `crypto/x509` provides an API to retrieve the bundle of
locally trusted certificates (see [`SystemCertPool()`](https://pkg.go.dev/crypto/x509#SystemCertPool)).

On Windows, where certificates are stored in the registry
[system stores](https://learn.microsoft.com/en-us/windows/win32/seccrypto/system-store-locations),
it's possible to get more fine grained access through the native APIs exposed
in package `golang.org/x/sys/windows` such as [`CertEnumCertificatesInStore`](https://learn.microsoft.com/en-us/windows/win32/api/wincrypt/nf-wincrypt-certenumcertificatesinstore)).

The certificates are extracted in the [PEM](https://www.rfc-editor.org/rfc/rfc1422) format.

### Installing the certificates in the guest OS

The list of certificates retrieved in the previous step is saved as a file in
the machine data folder,
`~/.local/share/containers/podman/machine/<provider>/<machine-name>`, using
[PEM](https://www.rfc-editor.org/rfc/rfc1422) encoding.

The file is then copied in the target machine under the path
`/etc/pki/ca-trust/source/anchors`. And the command `update-ca-trust`
is executed.

### If an error occurs during the import

The retrieval of the certificates, the creation of the certificates files, the
copy of the file from the host into the guest and the execution of the
`update-ca-trust` are all operations that can fail.

In the event of a failure, we warn the user but continue the machine startup
process.

## **Use cases**

### Sending requests to an HTTPS server in an enterprise network

The following example shows how to run an application inside a Podman container
that connects to an HTTPS server deployed within the internal enterprise
network. The server TLS certificate is signed by the enterprise CA.

```bash
podman run -ti --rm --security-opt=label=disable \
  -v /etc/ssl/certs/ca-certificates.crt:/etc/ssl/certs/ca-certificates.crt:ro \
  fedora curl -I https://internal.redhat.com
```

### Sending requests to an HTTPS server running on localhost

A developer may deploy a web application locally and use a custom x509
certificate signed by a locally trusted CA (using
[mkcert](https://github.com/FiloSottile/mkcert) for example).

They then need to run an application, in a container, that connects to the web
application. For example:

```bash
podman run -ti --rm --security-opt=label=disable \
  -v /etc/ssl/certs/ca-certificates.crt:/etc/ssl/certs/ca-certificates.crt:ro \
  fedora curl -I https://host.docker.internal:8080
```

## **Target Podman Release**

The target release is version 6.0.0.

## **Link(s)**

- [Jira issue](https://issues.redhat.com/browse/RUN-3839)

## **Stakeholders**

- [x] Podman Users
- [x] Podman Developers
- [ ] Buildah Users
- [ ] Buildah Developers
- [ ] Skopeo Users
- [ ] Skopeo Developers
- [x] Podman Desktop
- [ ] CRI-O
- [ ] Storage library
- [ ] Image library
- [ ] Common library
- [ ] Netavark and aardvark-dns

## **Assignee(s)**

- [@l0rd](https://github.com/l0rd)

## **Impacts**

### **CLI**

There will be a new flag `--import-native-ca` for the `podman machine start` command:

```bash
$ podman machine start --help
(...)
Options:
(...)
      --import-native-ca    Import the host trusted CA certificates into the machine

$ podman machine start --import-native-ca
Starting machine "podman-machine-default"
(...)
The host trusted CA certificates have been imported successfully
Machine "podman-machine-default" started successfully
```

### **Libpod**

None

### **Others**

Documentation needs to be updated:

- [Command: podman machine start](https://github.com/containers/podman/blob/main/docs/source/markdown/podman-machine-start.1.md.in)
- [Tutorial: Installing Crtificate Authority](https://github.com/containers/podman/blob/main/docs/tutorials/podman-install-certificate-authority.md)

## **Further Description (Optional):**

None

## **Test Descriptions (Optional):**

A new machine e2e tests will be added where we check that:

- when the flag `--import-native-ca` is passed, the local certificates are
imported.
- when the flag isn't passed the local certificates aren't imported.

Adding a custom CA certificate requires admin privileges on macOS and Windows
so we should avoid that. Instead we can

1) match the machine certificates with the host ones
2) look at the diff, in the machine certificates list, when the flag
`--import-native-ca` is turned on.
