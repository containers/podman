# SSH File Signatures

SSH keys can be used to sign files!
Unfortunately this is a pretty recent change to the openssh tooling, so it is not
supported by golang.org/x/crypto/ssh yet.

This document explains how it works at a high level.

## Keys

SSH keys are usually split into public and private files, named `id_rsa.pub` and
`id_rsa`, respectively.
These files are encoded and formatted a little differently than other signing keys.

### Public Keys

These are typically in the "known hosts" format.
This looks something like:

```
ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDw0ZWP4zZLELSJVenQTQsrFJVBnoP64KTg/UWRU6qOb8HEOdtHJDOyTmo9dvN/yJoTFtWAfQEjaTsMVJzTD0gOk6ncTsp0BUtgXawSCfEUiv7v+2VgSVbUfAv/NL+HEGSCdcORnansIyrZaHwAjR3ei3O+pRWvgjRj3pOH1rWGrxaC5IbsELYzS/HvwAG/uwcxgBv4POvaq6eCEHVbqRjIYjjoYsC+c24sgSQxOyXvDS7j2z9TPHPvepDhVr9y6xnnqhLqZEWmidRrbb35aYkVLJxmGTFy/JW1cewyU2Jb3+sKQOiOwL7DAB39tRyec2ed+EHh6QLW4pcMnoXsWuPyi+G595HiUYmIlqXJ5JPo0Cv/rOJrmWSFceWiDjC/SeODp/AcK0EsN/p3wOp6ac7EzAz9Npri0vwSQX4MUYlya/olKiKCx5GIhTZtXioREPd8v4osx2VrVyDxKX99PVVbxw1FXSe4u+PuOawJzUA4vW41mxUY9zoAsb/fvoNPtrrT9HfC+7Pg6ryBdz+445M8Atc8YjjLeYXkTXWD6KMielRzBFFoIwIgi0bMotq3iQ9IwjQSXPMDQLb+UPg8xqsgRsX3wvyZzdBhxO4Bdomv7JYmySysaGgliHktU8qRse1lpDIXMovPtowywcKL4U3seDKrq7saVO0qdsLavy1o0w== lorenc.d@gmail.com
```

These can be parsed with [ParseKnownHosts](https://pkg.go.dev/golang.org/x/crypto/ssh#ParseKnownHosts)
, NOT `ParsePublicKey`.

In addition to the key material itself, this can contain the algorithm (`ssh-rsa` here) and a comment
(lorenc.d@gmail.com) here.

### Private Keys

These are stored in an "armored" PEM format, resembling PGP or x509 keys:

```
-----BEGIN SSH PRIVATE KEY-----
<base64 encoded key here>
-----END SSH PRIVATE KEY-----
```

These can be parsed correctly with [ParsePrivateKey](https://pkg.go.dev/golang.org/x/crypto/ssh#ParsePrivateKey).

## Wire Format

The wire format is relatively standard.

* Bytes are laid out in order.
* Fixed-length fields are laid out at the proper offset with the specified length.
* Strings are stored with the size as a prefix.

## Signature

These can be generated and validated from the command line with the `ssh-keygen -Y` set of commands:
`sign`, `verify`, and `check-novalidate`.

To work with them in Go is a little tricker.
The signature is stored using a struct packed using the `openssh` wire format.
The data that is used in the signing function is also packed in another struct before it is signed.

### Signature Format

Signatures are formatted on disk in a PEM-encoded format.
The header is `-----BEGIN SSH SIGNATURE-----`, and the end is `-----END SSH SIGNATURE-----`.
The signature contents are base64-encoded.

The signature contents are wrapped with extra metadata, then encoded as a struct using the
`openssh` wire format.
That struct is defined [here](https://github.com/openssh/openssh-portable/blob/master/PROTOCOL.sshsig#L34).

In Go:

```
type WrappedSig struct {
	MagicHeader   [6]byte
	Version       uint32
	PublicKey     string
	Namespace     string
	Reserved      string
	HashAlgorithm string
	Signature     string
}
```

The `PublicKey` and `Signature` fields are also stored as openssh-wire-formatted structs.
The `MagicHeader` is `SSHSIG`.
The `Version` is 1.
The `Namespace` is `file` (for this use-case).
`Reserved` must be empty.

Go can already parse the `PublicKey` and `Signature` fields,
and the `Signature` struct contains a `Blob` with the signature data.

### Signed Message

In addition to these wrappers, the message to be signed is wrapped with some metadata before
it is passed to the signing function.

That wrapper is defined [here](https://github.com/openssh/openssh-portable/blob/master/PROTOCOL.sshsig#L81).

And in Go:

```
type MessageWrapper struct {
	Namespace     string
	Reserved      string
	HashAlgorithm string
	Hash          string
}
```.

So, the data must first be hashed, then packed in this struct and encoded in the
openssh wire format.
Then, this resulting data is signed using the desired signature function.

The `Namespace` field must be `file` (for this usecase).
The `Reserved` field must be empty.

The output of this signature function (and the hash) becomes the `Signature.Blob`
value, which gets wire-encoded, wrapped, wire-encoded and finally pem-encoded.
