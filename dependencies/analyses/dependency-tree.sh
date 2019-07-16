#!/usr/bin/bash

if test "$#" -ne 1; then
	echo "invalid arguments: usage: $0 path to package"
	exit 1
fi

go list $1/... \
	| xargs -d '\n' go list -f '{{ .ImportPath }}: {{ join .Imports ", " }}' \
	| awk '{ printf "%s\n\n", $0 }' \
	> direct-tree.tmp.$$ && mv -f direct-tree.tmp.$$ direct-tree.txt


go list $1/... \
	| xargs -d '\n' go list -f '{{ .ImportPath }}: {{ join .Deps ", " }}' \
	| awk '{ printf "%s\n\n", $0 }' \
	> transitive-tree.tmp.$$ && mv -f transitive-tree.tmp.$$ transitive-tree.txt
