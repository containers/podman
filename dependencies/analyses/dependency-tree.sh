#!/usr/bin/bash

if test "$#" -ne 1; then
	echo "invalid arguments: usage: $0 path to package"
	exit 1
fi

DATA=$(go list $1/... \
	| xargs -d '\n' go list -f '{{ .ImportPath }}: {{ join .Imports ", " }}' \
	| awk '{ printf "%s\n\n", $0 }' \
	)

echo "$DATA"
