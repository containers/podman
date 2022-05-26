# [errs] -- Error handling for Golang

[![check vulns](https://github.com/spiegel-im-spiegel/errs/workflows/vulns/badge.svg)](https://github.com/spiegel-im-spiegel/errs/actions)
[![lint status](https://github.com/spiegel-im-spiegel/errs/workflows/lint/badge.svg)](https://github.com/spiegel-im-spiegel/errs/actions)
[![GitHub license](https://img.shields.io/badge/license-Apache%202-blue.svg)](https://raw.githubusercontent.com/spiegel-im-spiegel/errs/master/LICENSE)
[![GitHub release](http://img.shields.io/github/release/spiegel-im-spiegel/errs.svg)](https://github.com/spiegel-im-spiegel/errs/releases/latest)

Package [errs] implements functions to manipulate error instances.
This package is required Go 1.13 or later.

## Usage

### Create new error instance with cause

```go
package main

import (
    "fmt"
    "os"

    "github.com/spiegel-im-spiegel/errs"
)

func checkFileOpen(path string) error {
    file, err := os.Open(path)
    if err != nil {
        return errs.New(
            "file open error",
            errs.WithCause(err),
            errs.WithContext("path", path),
        )
    }
    defer file.Close()

    return nil
}

func main() {
    if err := checkFileOpen("not-exist.txt"); err != nil {
        fmt.Printf("%v\n", err)             // file open error: open not-exist.txt: no such file or directory
        fmt.Printf("%#v\n", err)            // *errs.Error{Err:&errors.errorString{s:"file open error"}, Cause:&os.PathError{Op:"open", Path:"not-exist.txt", Err:0x2}, Context:map[string]interface {}{"function":"main.checkFileOpen", "path":"not-exist.txt"}}
        fmt.Printf("%+v\n", err)            // {"Type":"*errs.Error","Err":{"Type":"*errors.errorString","Msg":"file open error"},"Context":{"function":"main.checkFileOpen","path":"not-exist.txt"},"Cause":{"Type":"*os.PathError","Msg":"open not-exist.txt: no such file or directory","Cause":{"Type":"syscall.Errno","Msg":"no such file or directory"}}}
        fmt.Printf("%v\n", errs.Cause(err)) // no such file or directory
    }
}
```

### Wrapping error instance

```go
package main

import (
    "fmt"
    "os"

    "github.com/spiegel-im-spiegel/errs"
)

func checkFileOpen(path string) error {
    file, err := os.Open(path)
    if err != nil {
        return errs.Wrap(
            err,
            errs.WithContext("path", path),
        )
    }
    defer file.Close()

    return nil
}

func main() {
    if err := checkFileOpen("not-exist.txt"); err != nil {
        fmt.Printf("%v\n", err)             // open not-exist.txt: no such file or directory
        fmt.Printf("%#v\n", err)            // *errs.Error{Err:&os.PathError{Op:"open", Path:"not-exist.txt", Err:0x2}, Cause:<nil>, Context:map[string]interface {}{"function":"main.checkFileOpen", "path":"not-exist.txt"}}
        fmt.Printf("%+v\n", err)            // {"Type":"*errs.Error","Err":{"Type":"*os.PathError","Msg":"open not-exist.txt: no such file or directory","Cause":{"Type":"syscall.Errno","Msg":"no such file or directory"}},"Context":{"function":"main.checkFileOpen","path":"not-exist.txt"}}
        fmt.Printf("%v\n", errs.Cause(err)) // no such file or directory
    }
}
```

### Wrapping error instance with cause

```go
package main

import (
    "errors"
    "fmt"
    "os"

    "github.com/spiegel-im-spiegel/errs"
)

func checkFileOpen(path string) error {
    file, err := os.Open(path)
    if err != nil {
        return errs.Wrap(
            errors.New("file open error"),
            errs.WithCause(err),
            errs.WithContext("path", path),
        )
    }
    defer file.Close()

    return nil
}

func main() {
    if err := checkFileOpen("not-exist.txt"); err != nil {
        fmt.Printf("%v\n", err)             // file open error: open not-exist.txt: no such file or directory
        fmt.Printf("%#v\n", err)            // *errs.Error{Err:&errors.errorString{s:"file open error"}, Cause:&os.PathError{Op:"open", Path:"not-exist.txt", Err:0x2}, Context:map[string]interface {}{"function":"main.checkFileOpen", "path":"not-exist.txt"}}
        fmt.Printf("%+v\n", err)            // {"Type":"*errs.Error","Err":{"Type":"*errors.errorString","Msg":"file open error"},"Context":{"function":"main.checkFileOpen","path":"not-exist.txt"},"Cause":{"Type":"*os.PathError","Msg":"open not-exist.txt: no such file or directory","Cause":{"Type":"syscall.Errno","Msg":"no such file or directory"}}}
        fmt.Printf("%v\n", errs.Cause(err)) // no such file or directory
    }
}
```

[errs]: https://github.com/spiegel-im-spiegel/errs "spiegel-im-spiegel/errs: Error handling for Golang"
