# scp

[![Go Report Card](https://goreportcard.com/badge/github.com/dtylman/scp)](https://goreportcard.com/report/github.com/dtylman/scp)

A Simple `go` SCP client library.

## Usage

```go
import (
  "github.com/dtylman/scp"
  "golang.org/x/crypto/ssh"
)
```

## Sending Files

Copies `/var/log/messages` to remote `/tmp/lala`:

```go
var sc* ssh.Client
// establish ssh connection into sc here...
n,err:=scp.CopyTo(sc, "/var/log/messages", "/tmp/lala")
if err==nil{
  fmt.Printf("Sent %v bytes",n)
}
```

## Receiving Files

Copies remote `/var/log/message` to local `/tmp/lala`:

```go
var sc* ssh.Client
// establish ssh connection into sc here...
n,err:=scp.CopyFrom(sc, "/var/log/message", "/tmp/lala")
if err==nil{
  fmt.Printf("Sent %v bytes",n)
}
```


