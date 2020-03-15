#!/bin/bash

set -e

source $(dirname $0)/lib.sh

# Building this is a PITA, just grab binary for use in automation
# Ref: https://goswagger.io/install.html#static-binary
download_url=$(curl -s https://api.github.com/repos/go-swagger/go-swagger/releases/latest | \
  jq -r '.assets[] | select(.name | contains("'"$(uname | tr '[:upper:]' '[:lower:]')"'_amd64")) | .browser_download_url')
curl -o /usr/local/bin/swagger -L'#' "$download_url"
chmod +x /usr/local/bin/swagger

cd $GOSRC
make swagger
echo "Preserving build details for later use."
mv -v release.txt actual_release.txt  # Another 'make' during testing could overwrite it
