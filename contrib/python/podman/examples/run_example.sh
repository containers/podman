#!/bin/bash

export PYTHONPATH=..

function examples {
  for file in $@; do
    python3 -c "import ast; f=open('"${file}"'); t=ast.parse(f.read()); print(ast.get_docstring(t) + '  -- "${file}"')"
  done
}

while getopts "lh" arg; do
  case $arg in
    l ) examples $(ls eg_*.py); exit 0 ;;
    h ) echo 1>&2 $0 [-l] [-h] filename ; exit 2 ;;
  esac
done
shift $((OPTIND-1))

# podman needs to play some games with resources
if [[ $(id -u) != 0 ]]; then
  echo 1>&2 $0 must be run as root.
  exit 2
fi

if ! systemctl --quiet is-active io.podman.socket; then
  echo 1>&2 'podman is not running. systemctl enable --now io.podman.socket'
  exit 1
fi

function cleanup {
  podman rm $1 >/dev/null 2>&1
}

# Setup storage with an image and container
podman pull alpine:latest >/tmp/podman.output 2>&1
CTNR=$(podman create alpine)
trap "cleanup $CTNR" EXIT

if [[ -f $1 ]]; then
  python3 $1
else
  python3 $1.py
fi
