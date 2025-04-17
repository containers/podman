## iso9660
[![Go Reference](https://pkg.go.dev/badge/github.com/kdomanski/iso9660.svg)](https://pkg.go.dev/github.com/kdomanski/iso9660)
[![codecov](https://codecov.io/gh/kdomanski/iso9660/branch/master/graph/badge.svg?token=14MJSZYZ24)](https://codecov.io/gh/kdomanski/iso9660)
[![Go Report Card](https://goreportcard.com/badge/github.com/kdomanski/iso9660)](https://goreportcard.com/report/github.com/kdomanski/iso9660)

A package for reading and creating ISO9660

Joliet extension is **NOT** supported.

Experimental support for reading Rock Ridge extension is currently in the works.
If you are experiencing issues, please use the v0.3 release, which ignores Rock Ridge.

## References for the format:
- [ECMA-119 1st edition (December 1986)](https://www.ecma-international.org/wp-content/uploads/ECMA-119_1st_edition_december_1986.pdf) ([Web Archive link](http://web.archive.org/web/20210122025258/https://www.ecma-international.org/wp-content/uploads/ECMA-119_1st_edition_december_1986.pdf))
- [ECMA-119 2nd edition (December 1987)](https://www.ecma-international.org/wp-content/uploads/ECMA-119_2nd_edition_december_1987.pdf) ([Web Archive link](http://web.archive.org/web/20210418211711/https://www.ecma-international.org/wp-content/uploads/ECMA-119_2nd_edition_december_1987.pdf))
- [ECMA-119 3rd edition (December 2017)](https://www.ecma-international.org/wp-content/uploads/ECMA-119_3rd_edition_december_2017.pdf) ([Web Archive link](http://web.archive.org/web/20210527165925/https://www.ecma-international.org/wp-content/uploads/ECMA-119_3rd_edition_december_2017.pdf))
- [ECMA-119 4th edition (June 2019)](https://www.ecma-international.org/wp-content/uploads/ECMA-119_4th_edition_june_2019.pdf) ([Web Archive link](https://www.ecma-international.org/wp-content/uploads/ECMA-119_4th_edition_june_2019.pdf))
- [Rock Ridge Interchange Protocol](http://www.nextcomputers.org/NeXTfiles/Projects/CD-ROM/Rock_Ridge_Interchange_Protocol.pdf) ([Web Archive link](http://web.archive.org/web/20071017082049/http://www.nextcomputers.org/NeXTfiles/Projects/CD-ROM/Rock_Ridge_Interchange_Protocol.pdf))
- [System Use Sharing Protocol v1.12](http://aminet.net/package/docs/misc/RRIP)

## Examples

### Extracting an ISO

```go
package main

import (
  "log"

  "github.com/kdomanski/iso9660/util"
)

func main() {
  f, err := os.Open("/home/user/myImage.iso")
  if err != nil {
    log.Fatalf("failed to open file: %s", err)
  }
  defer f.Close()

  if err = util.ExtractImageToDirectory(f, "/home/user/target_dir"); err != nil {
    log.Fatalf("failed to extract image: %s", err)
  }
}
```

### Creating an ISO

```go
package main

import (
  "log"
  "os"

  "github.com/kdomanski/iso9660"
)

func main() {
  writer, err := iso9660.NewWriter()
  if err != nil {
    log.Fatalf("failed to create writer: %s", err)
  }
  defer writer.Cleanup()

  f, err := os.Open("/home/user/myFile.txt")
  if err != nil {
    log.Fatalf("failed to open file: %s", err)
  }
  defer f.Close()

  err = writer.AddFile(f, "folder/MYFILE.TXT")
  if err != nil {
    log.Fatalf("failed to add file: %s", err)
  }

  outputFile, err := os.OpenFile("/home/user/output.iso", os.O_WRONLY | os.O_TRUNC | os.O_CREATE, 0644)
  if err != nil {
    log.Fatalf("failed to create file: %s", err)
  }

  err = writer.WriteTo(outputFile, "testvol")
  if err != nil {
    log.Fatalf("failed to write ISO image: %s", err)
  }

  err = outputFile.Close()
  if err != nil {
    log.Fatalf("failed to close output file: %s", err)
  }
}
```

### Recursively create an ISO image from the given directories

```go
package main

import (
  "fmt"
  "log"
  "os"
  "path/filepath"
  "strings"

  "github.com/kdomanski/iso9660"
)

func main() {
  writer, err := iso9660.NewWriter()
  if err != nil {
    log.Fatalf("failed to create writer: %s", err)
  }
  defer writer.Cleanup()

  isoFile, err := os.OpenFile("C:/output.iso", os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
  if err != nil {
    log.Fatalf("failed to create file: %s", err)
  }
  defer isoFile.Close()

  prefix := "F:\\" // the prefix to remove in the output iso file
  sourceFolders := []string{"F:\\test1", "F:\\test2"} // the given directories to create an ISO file from

  for _, folderName := range sourceFolders {
    folderPath := strings.Join([]string{prefix, folderName}, "/")

    walk_err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
      if err != nil {
        log.Fatalf("walk: %s", err)
        return err
      }
      if info.IsDir() {
        return nil
      }
      outputPath := strings.TrimPrefix(path, prefix) // remove the source drive name
      fmt.Printf("Adding file: %s\n", outputPath)

      fileToAdd, err := os.Open(path)
      if err != nil {
        log.Fatalf("failed to open file: %s", err)
      }
      defer fileToAdd.Close()

      err = writer.AddFile(fileToAdd, outputPath)
      if err != nil {
        log.Fatalf("failed to add file: %s", err)
      }
      return nil
    })
    if walk_err != nil {
      log.Fatalf("%s", walk_err)
    }
  }

  err = writer.WriteTo(isoFile, "Test")
  if err != nil {
    log.Fatalf("failed to write ISO image: %s", err)
  }
}
```
