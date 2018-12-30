#!/bin/sh

# Generate go dependencies, for make. Uses `go list`.
# Usage: makedepend.sh output package path [extra]

PATH_FORMAT='{{ .ImportPath }}{{"\n"}}{{join .Deps "\n"}}'
FILE_FORMAT='{{ range $file := .GoFiles }} {{$.Dir}}/{{$file}}{{"\n"}}{{end}}'

out=$1
pkg=$2
path=$3
extra=$4

# check for mandatory parameters
test -n "$out$pkg$path" || exit 1

test -f "$path" && self=$path
echo "$out: $self $extra\\"
go list -f "$PATH_FORMAT" $path |
  grep "$pkg" |
  xargs go list -f "$FILE_FORMAT" |
  sed -e "s|^ ${GOPATH}| \$(GOPATH)|;s/$/ \\\\/"
echo " #"
